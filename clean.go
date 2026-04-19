package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type cleanItem struct {
	label string
	path  string
	kind  string // "file" or "dir"
}

// interactiveClean lists files in working directories and lets the user delete them.
// Mirrors Python interactive_clean().
func interactiveClean() {
	type target struct {
		base, pattern, kind string
	}
	targets := []target{
		{"input", "*.zip", "file"},
		{"input", "*.enex", "file"},
		{"input", "*", "dir"},
		{"output", "*.md", "file"},
		{"output", "images", "dir"},
		{"work_v01", "*", "dir"},
	}

	var items []cleanItem
	for _, t := range targets {
		if _, err := os.Stat(t.base); os.IsNotExist(err) {
			continue
		}
		switch t.kind {
		case "file":
			matches, _ := filepath.Glob(filepath.Join(t.base, t.pattern))
			sort.Strings(matches)
			for _, m := range matches {
				items = append(items, cleanItem{
					label: t.base + "/" + filepath.Base(m),
					path:  m,
					kind:  "file",
				})
			}
		case "dir":
			if t.pattern == "*" {
				entries, _ := os.ReadDir(t.base)
				for _, e := range entries {
					if e.IsDir() {
						items = append(items, cleanItem{
							label: t.base + "/" + e.Name() + "/",
							path:  filepath.Join(t.base, e.Name()),
							kind:  "dir",
						})
					}
				}
			} else {
				p := filepath.Join(t.base, t.pattern)
				if _, err := os.Stat(p); err == nil {
					items = append(items, cleanItem{
						label: t.base + "/" + t.pattern + "/",
						path:  p,
						kind:  "dir",
					})
				}
			}
		}
	}

	if len(items) == 0 {
		fmt.Println("Nothing to clean.")
		return
	}

	// Display grouped by base directory
	currentBase := ""
	for i, item := range items {
		base := strings.SplitN(item.label, "/", 2)[0]
		if base != currentBase {
			fmt.Printf("\n  [%s/]\n", base)
			currentBase = base
		}
		name := strings.SplitN(item.label, "/", 2)[1]
		fmt.Printf("    %d. %s\n", i+1, name)
	}

	fmt.Println("\nSelect to clean (space-separated numbers, 'a' for all, 'q' to quit):")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	choice := strings.TrimSpace(strings.ToLower(scanner.Text()))

	if choice == "q" {
		return
	}

	var selected []int
	if choice == "a" {
		for i := range items {
			selected = append(selected, i)
		}
	} else {
		// Accept comma or space separated numbers
		parts := strings.FieldsFunc(choice, func(r rune) bool {
			return r == ' ' || r == ','
		})
		for _, p := range parts {
			n, err := strconv.Atoi(strings.TrimSpace(p))
			if err != nil {
				fmt.Println("Invalid input.")
				return
			}
			selected = append(selected, n-1)
		}
	}

	for _, idx := range selected {
		if idx < 0 || idx >= len(items) {
			continue
		}
		item := items[idx]
		var err error
		if item.kind == "file" {
			err = os.Remove(item.path)
		} else {
			err = os.RemoveAll(item.path)
		}
		if err != nil {
			fmt.Printf("  Error: %s: %v\n", item.label, err)
		} else {
			fmt.Printf("  Deleted: %s\n", item.label)
		}
	}
}

// copyAndExtract copies a ZIP from output/ to dest, extracts it, then removes the ZIP.
// If zipFile and dest are empty, runs in interactive mode.
// Mirrors Python copy_and_extract().
func copyAndExtract(zipFile, dest string) error {
	outputBase := "output"

	if zipFile == "" {
		// Interactive: list ZIPs in output/
		zips, _ := filepath.Glob(filepath.Join(outputBase, "*.zip"))
		sort.Strings(zips)
		if len(zips) == 0 {
			fmt.Println("No ZIP files found in output/.")
			return nil
		}
		fmt.Println("\nSelect ZIP to copy and extract:")
		for i, z := range zips {
			fmt.Printf("  %d. %s\n", i+1, filepath.Base(z))
		}
		fmt.Print("\n> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		idx, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
		if err != nil || idx < 1 || idx > len(zips) {
			fmt.Println("Invalid selection.")
			return nil
		}
		zipFile = filepath.Base(zips[idx-1])
		fmt.Print("Destination path: ")
		scanner.Scan()
		dest = strings.TrimSpace(scanner.Text())
	}

	zipPath := filepath.Join(outputBase, zipFile)
	if filepath.IsAbs(zipFile) {
		zipPath = zipFile
	}
	if _, err := os.Stat(zipPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", zipPath)
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return err
	}

	// Copy ZIP to destination
	data, err := os.ReadFile(zipPath)
	if err != nil {
		return err
	}
	destZip := filepath.Join(dest, filepath.Base(zipPath))
	if err := os.WriteFile(destZip, data, 0644); err != nil {
		return err
	}

	// Extract then delete the copy
	if err := extractZip(destZip, dest); err != nil {
		return err
	}
	_ = os.Remove(destZip)

	fmt.Printf("✅ Extracted to: %s\n", dest)
	return nil
}
