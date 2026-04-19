package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

func convertEvernoteZip(inputZip, outputDir string) error {
	workDir := "work_v01"

	if err := resetDir(workDir); err != nil {
		return err
	}
	imagesDir := filepath.Join(outputDir, "images")

	// 1. Extract ZIP to work_v01/
	if err := extractZip(inputZip, workDir); err != nil {
		return fmt.Errorf("extract zip: %w", err)
	}

	// Find HTML file (skip __MACOSX metadata directories)
	var htmlFile string
	_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || htmlFile != "" {
			return err
		}
		if !info.IsDir() &&
			strings.ToLower(filepath.Ext(path)) == ".html" &&
			!strings.Contains(path, "__MACOSX") {
			htmlFile = path
		}
		return nil
	})
	if htmlFile == "" {
		return fmt.Errorf("HTML file not found in ZIP")
	}

	base := filepath.Base(htmlFile)
	originalTitle := strings.TrimSuffix(base, filepath.Ext(base))
	safeTitle := strings.ReplaceAll(originalTitle, " ", "_")

	// 2. Parse HTML
	// Read as string first so we can pre-flatten <div> wrappers inside table
	// cells before html.Parse (HTML5 foster-parenting moves them out of cells).
	rawHTML, err := os.ReadFile(htmlFile)
	if err != nil {
		return err
	}
	doc, err := html.Parse(strings.NewReader(preFlattenTableCells(string(rawHTML))))
	if err != nil {
		return fmt.Errorf("parse html: %w", err)
	}

	// 3. Handle <en-codeblock> (ZIP / HTML export format)
	// Structure: <en-codeblock><div data-plaintext="true">line</div>...</en-codeblock>
	codeblocks := findAll(doc, func(n *html.Node) bool {
		return isElement(n, "en-codeblock")
	})
	for _, cb := range codeblocks {
		var lines []string
		for c := cb.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && getAttr(c, "data-plaintext") == "true" {
				lines = append(lines, getText(c))
			}
		}
		replaceWith(cb, makePreCode(strings.TrimRight(strings.Join(lines, "\n"), "\n")))
	}

	// 4. Image handling: find referenced images in work_v01/, copy to output_v01/images/
	// and rename to {safe_title}_{counter:02d}{ext}. Update src attribute.
	imageCounter := 1
	imgNodes := findAll(doc, func(n *html.Node) bool {
		return isElement(n, "img")
	})
	for _, img := range imgNodes {
		src := getAttr(img, "src")
		if src == "" {
			continue
		}
		// Find the actual file in work_v01/ whose name appears in src
		var actualFile string
		_ = filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || actualFile != "" {
				return err
			}
			if !info.IsDir() &&
				strings.Contains(src, info.Name()) &&
				!strings.Contains(path, "__MACOSX") {
				actualFile = path
			}
			return nil
		})
		if actualFile == "" {
			continue
		}
		ext := filepath.Ext(actualFile)
		newName := fmt.Sprintf("%s_%02d%s", safeTitle, imageCounter, ext)
		data, err := os.ReadFile(actualFile)
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(imagesDir, newName), data, 0644); err != nil {
			return err
		}
		setAttr(img, "src", "images/"+newName)
		imageCounter++
	}

	// 5. Shared HTML → Markdown pipeline
	finalMd, err := htmlToMarkdown(doc, originalTitle)
	if err != nil {
		return err
	}

	// 6. Write output
	mdPath := filepath.Join(outputDir, safeTitle+".md")
	if err := os.WriteFile(mdPath, []byte(finalMd), 0644); err != nil {
		return err
	}

	return nil
}

// extractZip extracts all files from zipPath into destDir.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(destDir, f.Name)

		// Guard against zip slip
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(fpath, f.Mode())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(fpath)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
