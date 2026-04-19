# ev2md

## Table of Contents

- [Overview](#overview)
- [Background](#background)
- [Files](#files)
- [Usage](#usage)
- [Processing Pipeline](#processing-pipeline)
  - [ENEX pipeline](#enex-pipeline)
  - [HTML ZIP pipeline](#html-zip-pipeline)
  - [Shared markdown pipeline](#shared-markdown-pipeline)
- [Known Pitfalls](#known-pitfalls)
  - [`<en-media>` content loss](#en-media-content-loss-htmlutilgo-unwrapchildren)
  - [TOC anchor links](#toc-anchor-links-markdowngo-step-56)
  - [Markdown escape sequences](#markdown-escape-sequences-markdowngo-step-9)
  - [HTML table conversion](#html-table-conversion-htmlutilgo-flattentablecells-markdowngo-step-12)
- [Working Directories](#working-directories)
- [Dependencies](#dependencies)
- [Build & Install](#build--install)

---

## Overview

`ev2md` converts Evernote exports (ENEX or HTML ZIP) into Markdown documents with properly managed image assets.  
Output is `.md` files and an `images/` directory written directly to the output directory.  

[↑ Table of Contents](#table-of-contents)

---

## Background

This is a Go port of the Docker-based Python tool at `/Users/junsu.lee/software/podman/evernote2md/`.  
The original Python version used `beautifulsoup4` + `markdownify` inside a Podman container.  
This Go version produces identical output as a single binary — no container required.  

**Original Python tool commands (for reference):**

| Alias | Command |
|-------|---------|
| `ev2mdbld` | `podman compose build --no-cache` |
| `ev2mdup` | `docker-compose up -d` |
| `ev2md` | `podman exec -it evernote2md bash` (enter container) |

[↑ Table of Contents](#table-of-contents)

---

## Files

| File | Description |
|------|-------------|
| `main.go` | CLI parsing (`parseArgs`), dispatch, `resetDir`, `zipDirectory` |
| `htmlutil.go` | HTML tree helpers: `findAll`, `removeNodes`, `replaceWith`, `getText`, etc. |
| `markdown.go` | `htmlToMarkdown()` — shared post-processing pipeline |
| `enex.go` | `convertEnex()` — ENEX (XML) input path |
| `htmlzip.go` | `convertEvernoteZip()` — HTML ZIP input path, `extractZip()` |
| `clean.go` | `interactiveClean()`, `copyAndExtract()` |
| `Makefile` | Cross-platform build and install |

[↑ Table of Contents](#table-of-contents)

---

## Usage

```bash
ev2md input/test.enex                         # ENEX → output/
ev2md input/test.zip                          # HTML ZIP → output/
ev2md input/test.enex -o /tmp/out             # custom output directory
ev2md input/                                  # all .enex/.zip in directory → output/
ev2md test/ -o test/output                    # directory input with custom output
ev2md -c                                      # interactive clean
ev2md -cp                                     # interactive copy+extract
ev2md -cp test_converted.zip /path/to/dest    # copy+extract to path
```

Run from the directory containing `input/`, `output/`, `output_v01/`, `work_v01/` (same as Python version).  

[↑ Table of Contents](#table-of-contents)

---

## Processing Pipeline

### ENEX pipeline

(`enex.go: convertEnex`)

1. Read ENEX file, strip DOCTYPE (prevents xml.Unmarshal from fetching external DTD)
2. Parse XML → extract `<note>` elements
3. For each note: decode base64 `<resource>` data, build MD5 hash → `{data, ext}` map
4. Strip `<div>` wrappers inside table cells (`preFlattenTableCells`) — must run before `html.Parse` due to HTML5 foster parenting (see Known Pitfalls); then parse ENML with `html.Parse`
5. Replace `<en-media hash="...">` with `<img src="images/...">` using hash map
6. Convert code blocks:
   - New format: `<div style="--en-codeblock:true">` with child divs per line → `<pre><code>`
   - Old format: `<en-codeblock>` with `data-plaintext="true"` divs → `<pre><code>`
7. Run shared markdown pipeline → `finalMd`
8. If note has `<tag>` elements, insert `## Tags` section after the `# Title` line
9. Write `{safe_title}.md` to `output_v01/`

[↑ Table of Contents](#table-of-contents)

---

### HTML ZIP pipeline

(`htmlzip.go: convertEvernoteZip`)

1. Extract ZIP to `work_v01/`
2. Find `.html` file (skip `__MACOSX` metadata)
3. Strip `<div>` wrappers inside table cells (`preFlattenTableCells`) then parse HTML with `html.Parse`
4. Convert `<en-codeblock>` → `<pre><code>` (same old-format logic as ENEX)
5. For each `<img>`: find referenced file in `work_v01/`, copy to `output_v01/images/` as `{safe_title}_{n:02d}{ext}`, update `src`
6. Run shared markdown pipeline → write `{safe_title}.md`

[↑ Table of Contents](#table-of-contents)

---

### Shared markdown pipeline

(`markdown.go: htmlToMarkdown`)

1. Strip `<style>`, `<script>`, `<meta>`, `<link>` tags from the HTML tree
2. Serialize body to HTML string, convert to Markdown via `html-to-markdown` (ATX headings, GFM tables); insert missing `| --- |` separator rows (`addTableSeparators`)
3. Collapse 3+ blank lines to 2
4. Demote all headings by 1 level (H1→H2, …, H5→H6); skip lines inside fenced code blocks
5. Build TOC from H2 (top-level) and H3 (indented) headings
6. Insert `[contents](#contents)` backlink before each H2/H3/H4 heading when content has been seen since the previous heading
7. Prepend `# {title}\n\n## Contents\n{toc}\n`
8. Convert MDX-incompatible autolinks: `<https://...>` → `[https://...](https://...)`
9. Restore escaped chars (`\-` `\*` `\_` `\|`) inside and outside fenced code blocks

[↑ Table of Contents](#table-of-contents)

---

## Known Pitfalls

### `<en-media>` content loss (`htmlutil.go: unwrapChildren`)

The Go HTML5 parser (`golang.org/x/net/html`) treats `<en-media ... />` as a regular opening tag, not self-closing. Everything between `<en-media>` and the end of its parent block ends up nested as its children. Calling `RemoveChild` or `replaceWith` on the node therefore silently drops all note content that follows the image in the same block — entire sections can disappear.

**Fix:** `unwrapChildren(n)` is called before every remove/replace on `<en-media>` nodes. It re-parents the children as siblings, preserving the content.

### Markdown escape sequences (`markdown.go` step 9)

The html-to-markdown library escapes `\-` `\*` `\_` `\|` to prevent misinterpretation during rendering. ServiceNow KB displays these escape sequences literally instead of stripping the backslash. They are removed in the final pass both inside and outside code blocks.

### HTML table conversion (`htmlutil.go: preFlattenTableCells`, `markdown.go: addTableSeparators`)

Evernote ENEX wraps table cell content in `<div>` tags (e.g. `<th><div><a href="...">col</a></div></th>`).

**Problem 1 — cells rendered as paragraphs:**
The HTML5 parser (`golang.org/x/net/html`) applies "foster parenting": `<div>` inside `<th>`/`<td>` is illegal block content, so the parser moves it out of the table entirely before the tree is built. Any post-parse fix (`flattenTableCells`) therefore has no effect — the `<div>` is already gone from the cell by the time the function runs.

**Fix:** `preFlattenTableCells(htmlStr)` strips `<div[^>]*>` / `</div>` immediately adjacent to `<th>`/`<td>` tags in the raw HTML *string*, before `html.Parse` is called (in `enex.go` and `htmlzip.go`).

**Problem 2 — missing `| --- |` separator row:**
When `<th>` cells contain inline elements (e.g. `<a>` links), the `html-to-markdown` library emits the header row but omits the required GFM separator row. Without `| --- |`, the table is not recognised as a table by any renderer.

**Fix:** `addTableSeparators(md)` scans the markdown output after conversion and inserts `| --- | ... |` after any header row that is immediately followed by a data row with no separator.

**Also required:** `converter.Use(plugin.Table())` enables the GFM table plugin — without it the library falls back to `TableCompat` which outputs cells separated by ` · ` with no `|` syntax at all.

[↑ Table of Contents](#table-of-contents)

---

## Working Directories

| Directory | Purpose |
|-----------|---------|
| `input/` | Input `.enex` or `.zip` files |
| `output/` | Final output ZIPs (`{stem}_converted.zip`) |
| `output_v01/` | Intermediate: `.md` files + `images/` |
| `work_v01/` | Temporary extraction for ZIP input (cleared on each run) |

`output_v01/` and `work_v01/` are cleared and recreated on each run.  

[↑ Table of Contents](#table-of-contents)

---

## Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/JohannesKaufmann/html-to-markdown v1.6.0` | HTML → Markdown conversion (ATX headings, GFM tables via `plugin.Table()`) |
| `golang.org/x/net` | `html.Parse` for parsing HTML/ENML |
| stdlib: `archive/zip`, `encoding/xml`, `encoding/base64`, `crypto/md5` | ZIP, XML, ENEX image handling |

[↑ Table of Contents](#table-of-contents)

---

## Build & Install

```bash
make          # clean + build all platforms
make build    # build only
make install  # copy darwin binary to ~/bin/ev2md
make clean    # remove build/ directory
```

Output paths:

```
build/linux_amd64/ev2md
build/darwin_amd64/ev2md
build/darwin_arm64/ev2md
build/windows_amd64/ev2md.exe
```

[↑ Table of Contents](#table-of-contents)
