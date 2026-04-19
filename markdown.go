package main

import (
	"fmt"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"golang.org/x/net/html"
)

var (
	reBlankLines  = regexp.MustCompile(`\n{3,}`)
	reHeading     = regexp.MustCompile(`^#{1,5} `)
	reH23         = regexp.MustCompile(`^(#{2,3}) (.+)`)
	reH2to4       = regexp.MustCompile(`^(#{2,4}) (.+)`)
	reH2          = regexp.MustCompile(`^## (.+)`)
	reAnchorStrip = regexp.MustCompile(`[^\w\- ]`)
	reAutolink    = regexp.MustCompile(`<(https?://[^>]+)>`)
	reCodeBlock   = regexp.MustCompile("(?s)```.*?```")
)

// isTableRow returns true if line looks like a GFM table row (starts and ends with |).
func isTableRow(line string) bool {
	t := strings.TrimSpace(line)
	return len(t) > 2 && strings.HasPrefix(t, "|") && strings.HasSuffix(t, "|")
}

// isSeparatorRow returns true if line is a GFM table separator (only |, -, :, spaces).
func isSeparatorRow(line string) bool {
	if !isTableRow(line) {
		return false
	}
	t := strings.TrimSpace(line)
	inner := t[1 : len(t)-1]
	for _, c := range inner {
		if c != '|' && c != '-' && c != ':' && c != ' ' {
			return false
		}
	}
	return true
}

// countTableCols counts the number of columns in a table row.
func countTableCols(line string) int {
	t := strings.TrimSpace(line)
	if strings.HasPrefix(t, "|") {
		t = t[1:]
	}
	if strings.HasSuffix(t, "|") {
		t = t[:len(t)-1]
	}
	return len(strings.Split(t, "|"))
}

// addTableSeparators inserts | --- | rows after header rows that are missing them.
// html-to-markdown omits the separator when <th> cells contain inline elements
// like <a> tags instead of plain text.
func addTableSeparators(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	inCode := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
		}
		out = append(out, line)
		if inCode {
			continue
		}
		if isTableRow(line) {
			prevIsTable := i > 0 && isTableRow(lines[i-1])
			nextIsData := i+1 < len(lines) && isTableRow(lines[i+1]) && !isSeparatorRow(lines[i+1])
			if !prevIsTable && nextIsData {
				ncols := countTableCols(lines[i+1])
				out = append(out, "|"+strings.Repeat(" --- |", ncols))
			}
		}
	}
	return strings.Join(out, "\n")
}

// makeAnchor converts a heading string to a GitHub-style anchor ID.
func makeAnchor(s string) string {
	a := strings.ToLower(reAnchorStrip.ReplaceAllString(s, ""))
	return strings.ReplaceAll(a, " ", "-")
}

// htmlToMarkdown converts a parsed HTML node to a fully processed Markdown string.
// Mirrors the Python _html_to_markdown() pipeline:
//  1. Strip style/script/meta/link tags
//  2. HTML → Markdown (ATX headings, GFM tables); insert missing | --- | separator rows
//  3. Collapse 3+ blank lines to 2
//  4. Demote all headings by 1 level (skip code blocks)
//  5. Build TOC from H2/H3 headings
//  6. Insert [contents](#contents) backlinks before H2/H3/H4 headings
//  7. Prepend "# title\n\n## Contents\n{toc}\n"
//  8. Convert MDX-incompatible autolinks
//  9. Restore escaped chars (inside and outside code blocks)
func htmlToMarkdown(doc *html.Node, title string) (string, error) {

	// 1. Strip unwanted tags
	removeNodes(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		switch n.Data {
		case "style", "script", "meta", "link":
			return true
		}
		return false
	})

	// flattenTableCells is a post-parse safety net; the real fix is
	// preFlattenTableCells() called before html.Parse in enex.go / htmlzip.go.
	// HTML5 foster parenting moves <div> out of <th>/<td> before the tree is
	// built, so this post-parse pass has no effect on ENEX/ZIP input but is
	// kept here in case a caller passes already-clean HTML.
	flattenTableCells(doc)

	// Serialize body (or full doc) to HTML string for conversion
	htmlStr := renderNode(bodyOrDoc(doc))

	// 2. HTML → Markdown (ATX headings, GFM tables).
	// plugin.Table() is required for GFM | col | format; without it the library
	// falls back to TableCompat which outputs cells separated by ' · '.
	converter := md.NewConverter("", true, &md.Options{
		HeadingStyle: "atx",
	})
	converter.Use(plugin.Table())
	markdownContent, err := converter.ConvertString(htmlStr)
	if err != nil {
		return "", fmt.Errorf("markdown conversion: %w", err)
	}
	// When <th> cells contain inline elements (e.g. <a> links), the library
	// emits the header row but omits the required | --- | separator row.
	// addTableSeparators inserts the missing separator after any such header row.
	markdownContent = addTableSeparators(markdownContent)

	// 3. Collapse 3+ blank lines to 2.
	markdownContent = reBlankLines.ReplaceAllString(markdownContent, "\n\n")

	// 4. Demote all headings by 1 level (H1→H2, H2→H3, ..., H5→H6).
	// Lines inside fenced code blocks are skipped to avoid treating
	// comment lines like "# some text" as headings.
	lines := strings.Split(markdownContent, "\n")
	inCode := false
	for i, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
		}
		if !inCode && reHeading.MatchString(line) {
			lines[i] = "#" + line
		}
	}
	markdownContent = strings.Join(lines, "\n")

	// 5. Build TOC from H2 (top-level) and H3 (indented) headings
	var tocLines []string
	inCode = false
	for _, line := range strings.Split(markdownContent, "\n") {
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		m := reH23.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		level := len(m[1])
		sec := strings.TrimSpace(m[2])
		if strings.ToLower(sec) == "contents" {
			continue
		}
		indent := strings.Repeat("  ", level-2)
		tocLines = append(tocLines, fmt.Sprintf("%s- [%s](#%s)", indent, sec, makeAnchor(sec)))
	}
	contents := "## Contents\n" + strings.Join(tocLines, "\n") + "\n"

	// 6. Insert [contents](#contents) backlinks before H2/H3/H4 headings.
	//    A backlink is inserted only when content has been seen since the previous
	//    heading — avoids double backlinks on consecutive headings with no content.
	lines = strings.Split(markdownContent, "\n")
	var newLines []string
	contentSeen := false
	inCode = false
	for _, line := range lines {
		if strings.HasPrefix(line, "```") {
			inCode = !inCode
		}
		if !inCode {
			if hm := reH2to4.FindStringSubmatch(line); hm != nil {
				heading := strings.TrimSpace(hm[2])
				isContents := strings.ToLower(heading) == "contents"
				if !isContents && contentSeen {
					newLines = append(newLines, "", "[contents](#contents)", "")
				}
				contentSeen = false
				newLines = append(newLines, line)
				continue
			}
			if strings.TrimSpace(line) != "" {
				contentSeen = true
			}
		}
		newLines = append(newLines, line)
	}
	if contentSeen {
		newLines = append(newLines, "", "[contents](#contents)", "")
	}

	// 7. Prepend title + TOC.
	finalMd := fmt.Sprintf("# %s\n\n%s\n", title, contents) + strings.Join(newLines, "\n")

	// 8. Convert MDX-incompatible autolinks: <https://...> → [https://...](https://...)
	finalMd = reAutolink.ReplaceAllString(finalMd, "[$1]($1)")

	// 9. Restore html-to-markdown escaped chars inside and outside code blocks.
	//    The html-to-markdown library escapes \- \* \_ \| to prevent markdown
	//    misinterpretation (e.g. \- at line start avoids creating a list item).
	//    Inside code blocks these escapes are purely wrong — code is verbatim.
	//    Outside code blocks ServiceNow KB renders them literally (\- shows as \-)
	//    instead of stripping the backslash, so they must be removed here too.
	var mdEscapes = [][2]string{
		{`\-`, "-"}, {`\*`, "*"}, {`\_`, "_"}, {`\|`, "|"},
	}
	codeBlocks := reCodeBlock.FindAllString(finalMd, -1)
	parts := reCodeBlock.Split(finalMd, -1)
	var sb strings.Builder
	for i, part := range parts {
		for _, e := range mdEscapes {
			part = strings.ReplaceAll(part, e[0], e[1])
		}
		sb.WriteString(part)
		if i < len(codeBlocks) {
			block := codeBlocks[i]
			for _, e := range mdEscapes {
				block = strings.ReplaceAll(block, e[0], e[1])
			}
			sb.WriteString(block)
		}
	}
	finalMd = sb.String()

	return finalMd, nil
}
