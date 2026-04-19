package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// ENEX XML structure
type enexRoot struct {
	Notes []enexNote `xml:"note"`
}

type enexNote struct {
	Title     string     `xml:"title"`
	Tags      []string   `xml:"tag"`
	Content   string     `xml:"content"`
	Resources []enexRes  `xml:"resource"`
}

type enexRes struct {
	Data string `xml:"data"`
	Mime string `xml:"mime"`
}

var mimeToExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"image/bmp":  ".bmp",
}

var (
	reDoctypeStrip = regexp.MustCompile(`(?i)<!DOCTYPE[^>]+>`)
	reXMLDecl      = regexp.MustCompile(`<\?xml[^>]+\?>`)
	reEnCBStyle    = regexp.MustCompile(`--en-codeblock:\s*true`)
)

func convertEnex(inputEnex, outputDir string) error {
	imagesDir := filepath.Join(outputDir, "images")

	// 1. Read and strip DOCTYPE (prevents external DTD fetch by xml.Unmarshal)
	raw, err := os.ReadFile(inputEnex)
	if err != nil {
		return fmt.Errorf("read enex: %w", err)
	}
	cleaned := reDoctypeStrip.ReplaceAll(raw, nil)

	// 2. Parse ENEX XML
	var root enexRoot
	if err := xml.Unmarshal(cleaned, &root); err != nil {
		return fmt.Errorf("parse enex xml: %w", err)
	}
	if len(root.Notes) == 0 {
		return fmt.Errorf("no notes found in ENEX file")
	}

	imageCounter := 1 // global counter for unique filenames across all notes

	for _, note := range root.Notes {
		title := note.Title
		if title == "" {
			title = "untitled"
		}
		safeTitle := strings.ReplaceAll(title, " ", "_")

		// 3. Build MD5 hash → resource map from base64-encoded <resource> data
		type resEntry struct {
			data []byte
			ext  string
		}
		hashMap := map[string]resEntry{}
		for _, res := range note.Resources {
			if res.Data == "" {
				continue
			}
			ext := mimeToExt[res.Mime]
			if ext == "" {
				ext = ".bin"
			}
			rawData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(res.Data))
			if err != nil {
				continue
			}
			hash := fmt.Sprintf("%x", md5.Sum(rawData))
			hashMap[hash] = resEntry{rawData, ext}
		}

		// 4. Parse ENML from <content> CDATA
		// Strip XML declaration and DOCTYPE so html.Parse handles it cleanly
		enml := reXMLDecl.ReplaceAllString(note.Content, "")
		enml = reDoctypeStrip.ReplaceAllString(enml, "")
		// Pre-flatten <div> wrappers inside table cells before html.Parse.
		// The HTML5 parser foster-parents block-level <div> out of <th>/<td>,
		// making post-parse cleanup impossible.
		enml = preFlattenTableCells(enml)

		doc, err := html.Parse(strings.NewReader(enml))
		if err != nil {
			return fmt.Errorf("parse enml (note %q): %w", title, err)
		}

		// 5. Replace <en-media hash="..."> with <img src="images/...">
		enMediaNodes := findAll(doc, func(n *html.Node) bool {
			return isElement(n, "en-media")
		})
		for _, n := range enMediaNodes {
			hashVal := getAttr(n, "hash")
			// unwrapChildren must be called before any remove/replace.
			// See htmlutil.go: unwrapChildren for the full explanation.
			unwrapChildren(n)
			if res, ok := hashMap[hashVal]; ok {
				newName := fmt.Sprintf("%s_%02d%s", safeTitle, imageCounter, res.ext)
				if err := os.WriteFile(filepath.Join(imagesDir, newName), res.data, 0644); err != nil {
					return err
				}
				replaceWith(n, newImg("images/"+newName))
				imageCounter++
			} else {
				n.Parent.RemoveChild(n) // remove unresolved media
			}
		}

		// 6a. Code blocks — new ENEX format: <div style="--en-codeblock:true; ...">
		// Direct child <div>s are one line each; <div><br/></div> → blank line
		codeNewFmt := findAll(doc, func(n *html.Node) bool {
			return isElement(n, "div") && reEnCBStyle.MatchString(getAttr(n, "style"))
		})
		for _, cb := range codeNewFmt {
			lines := getDirectChildTexts(cb)
			replaceWith(cb, makePreCode(strings.TrimRight(strings.Join(lines, "\n"), "\n")))
		}

		// 6b. Code blocks — old ENEX format: <en-codeblock> with data-plaintext divs
		codeOldFmt := findAll(doc, func(n *html.Node) bool {
			return isElement(n, "en-codeblock")
		})
		for _, cb := range codeOldFmt {
			var lines []string
			for c := cb.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && getAttr(c, "data-plaintext") == "true" {
					lines = append(lines, getText(c))
				}
			}
			replaceWith(cb, makePreCode(strings.TrimRight(strings.Join(lines, "\n"), "\n")))
		}

		// 7. Shared HTML → Markdown pipeline
		finalMd, err := htmlToMarkdown(doc, title)
		if err != nil {
			return fmt.Errorf("note %q: %w", title, err)
		}

		// 8. Insert ## Tags section after title line (ENEX-only)
		if len(note.Tags) > 0 {
			var sb strings.Builder
			sb.WriteString("## Tags\n")
			for _, tag := range note.Tags {
				fmt.Fprintf(&sb, "- TAG_%s\n", tag)
			}
			sb.WriteString("\n")
			// Split on first newline to isolate "# Title" line
			parts := strings.SplitN(finalMd, "\n", 3)
			if len(parts) >= 2 {
				finalMd = parts[0] + "\n\n" + sb.String() + strings.Join(parts[1:], "\n")
			}
		}

		// 9. Write .md file for this note
		mdPath := filepath.Join(outputDir, safeTitle+".md")
		if err := os.WriteFile(mdPath, []byte(finalMd), 0644); err != nil {
			return err
		}
	}

	return nil
}
