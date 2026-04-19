package main

import (
	"bytes"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	reTableCellDivOpen  = regexp.MustCompile(`(<t[dh][^>]*>)<div[^>]*>`)
	reTableCellDivClose = regexp.MustCompile(`</div>(</t[dh]>)`)
)

// preFlattenTableCells removes single <div> wrappers directly inside <th>/<td>
// in the raw HTML string, BEFORE calling html.Parse.
//
// Why pre-parse: the HTML5 parser's "foster parenting" rule moves block-level
// <div> elements found inside table cells to before the table. By the time
// flattenTableCells (post-parse) runs, the <div> nodes are already outside the
// cells and the function has no effect.
//
// This pre-pass strips the outer <div> wrapper at the string level so the
// parser sees only inline content inside each cell and preserves table structure.
func preFlattenTableCells(htmlStr string) string {
	htmlStr = reTableCellDivOpen.ReplaceAllString(htmlStr, "$1")
	htmlStr = reTableCellDivClose.ReplaceAllString(htmlStr, "$1")
	return htmlStr
}

// renderNode serializes an HTML node back to a string.
func renderNode(n *html.Node) string {
	var buf bytes.Buffer
	_ = html.Render(&buf, n)
	return buf.String()
}

// removeNodes removes all nodes matching pred from the tree.
// Collects targets first to avoid modifying the tree while walking.
func removeNodes(n *html.Node, pred func(*html.Node) bool) {
	var targets []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if pred(node) {
			targets = append(targets, node)
			return // skip children of removed nodes
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	for _, node := range targets {
		node.Parent.RemoveChild(node)
	}
}

// findAll returns all nodes matching pred (collected before any modification).
func findAll(n *html.Node, pred func(*html.Node) bool) []*html.Node {
	var result []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if pred(node) {
			result = append(result, node)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return result
}

// getAttr returns the value of a named attribute on an element node.
func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// setAttr sets (or adds) a named attribute on an element node.
func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if a.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

// getText returns all text content within a node (recursive).
func getText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			sb.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

// getDirectChildTexts returns text content of direct element children only.
// Used for extracting code lines from en-codeblock divs.
func getDirectChildTexts(n *html.Node) []string {
	var lines []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			lines = append(lines, getText(c))
		}
	}
	return lines
}

// replaceWith replaces old with newNode in old's parent.
func replaceWith(old, newNode *html.Node) {
	parent := old.Parent
	parent.InsertBefore(newNode, old)
	parent.RemoveChild(old)
}

// unwrapChildren moves all children of n to be siblings immediately after n.
//
// Root cause: the HTML5 parser (golang.org/x/net/html) treats <en-media ... />
// as a regular opening tag, not a self-closing one (only void elements like <img>
// get that treatment). So everything between <en-media> and the end of its parent
// ends up nested inside <en-media> as children.
//
// Consequence: replaceWith / RemoveChild on an <en-media> node drops those children,
// silently erasing all note content that follows the image in the same block.
//
// Fix: call unwrapChildren before removing or replacing any <en-media> node so its
// children are re-parented as siblings and preserved in the document.
func unwrapChildren(n *html.Node) {
	if n.Parent == nil || n.FirstChild == nil {
		return
	}
	parent := n.Parent
	next := n.NextSibling
	for n.FirstChild != nil {
		child := n.FirstChild
		n.RemoveChild(child)
		if next != nil {
			parent.InsertBefore(child, next)
		} else {
			parent.AppendChild(child)
		}
	}
}

// newElement creates a new element node.
func newElement(tag string) *html.Node {
	return &html.Node{
		Type: html.ElementNode,
		Data: tag,
	}
}

// newTextNode creates a new text node.
func newTextNode(text string) *html.Node {
	return &html.Node{
		Type: html.TextNode,
		Data: text,
	}
}

// newImg creates a new <img src="..."> node.
func newImg(src string) *html.Node {
	return &html.Node{
		Type: html.ElementNode,
		Data: "img",
		Attr: []html.Attribute{{Key: "src", Val: src}},
	}
}

// isElement checks if node is an element with the given tag name.
func isElement(n *html.Node, tag string) bool {
	return n.Type == html.ElementNode && n.Data == tag
}

// hasAttrContaining checks if an attribute value contains a substring.
func hasAttrContaining(n *html.Node, key, substr string) bool {
	return strings.Contains(getAttr(n, key), substr)
}

// makePreCode builds a <pre><code>text</code></pre> subtree.
func makePreCode(codeText string) *html.Node {
	pre := newElement("pre")
	code := newElement("code")
	code.AppendChild(newTextNode(codeText))
	pre.AppendChild(code)
	return pre
}

// flattenTableCells removes <div> and <p> wrappers that are direct children of
// <th> or <td> elements. Evernote ENEX exports wrap cell content in <div> tags
// (e.g. <th><div><a href="...">col</a></div></th>). The html-to-markdown library
// treats these block-level wrappers as paragraph breaks, so each cell is output
// as a separate paragraph instead of a table column. Unwrapping the divs makes
// the cell content inline, which the library converts to a proper | col | row.
func flattenTableCells(doc *html.Node) {
	cells := findAll(doc, func(n *html.Node) bool {
		return isElement(n, "th") || isElement(n, "td")
	})
	for _, cell := range cells {
		var wrappers []*html.Node
		for c := cell.FirstChild; c != nil; c = c.NextSibling {
			if isElement(c, "div") || isElement(c, "p") {
				wrappers = append(wrappers, c)
			}
		}
		for _, w := range wrappers {
			next := w.NextSibling
			for w.FirstChild != nil {
				child := w.FirstChild
				w.RemoveChild(child)
				if next != nil {
					cell.InsertBefore(child, next)
				} else {
					cell.AppendChild(child)
				}
			}
			cell.RemoveChild(w)
		}
	}
}

// bodyOrDoc returns the <body> node if present, otherwise doc itself.
func bodyOrDoc(doc *html.Node) *html.Node {
	bodies := findAll(doc, func(n *html.Node) bool {
		return isElement(n, "body")
	})
	if len(bodies) > 0 {
		return bodies[0]
	}
	return doc
}
