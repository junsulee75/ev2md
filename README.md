# ev2md

Convert Evernote exports (ENEX or HTML ZIP) into Markdown documents.  

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [Options](#options)
- [Processing Pipeline](#processing-pipeline)
- [Known Pitfalls](#known-pitfalls)
- [Dependencies](#dependencies)

---

## Overview

`ev2md` converts Evernote ENEX exports or Evernote HTML ZIP exports into Markdown files with properly managed image assets.  
Output is `.md` files and an `images/` directory written to the specified output directory.  
Single file and batch (directory) conversion are both supported.  

Suggestions to: junsu.lee@servicenow.com  

[↑ Table of Contents](#table-of-contents)

---

## Installation

### Prerequisites

- Go 1.21 or later: https://go.dev/dl/

### Build from source

```bash
git clone https://github.com/junsulee75/ev2md.git
cd ev2md
make build    # builds all platforms under build/
make install  # installs darwin binary to ~/bin/ev2md
```

Build output:

```
build/linux_amd64/ev2md
build/darwin_amd64/ev2md
build/darwin_arm64/ev2md
build/windows_amd64/ev2md.exe
```

[↑ Table of Contents](#table-of-contents)

---

## Usage

```bash
# Convert a single ENEX file → written to ./output/
ev2md notes/my_note.enex

# Convert a single ENEX file to a specific output directory
ev2md notes/my_note.enex -o /tmp/out

# Convert all .enex/.zip files in a directory → written to ./output/
ev2md notes/

# Convert all files in a directory with a specific output path
ev2md test/ -o test/output

# Same, but clear output directory first (-r)
ev2md test/ -o test/output -r

# Interactive cleanup of generated directories
ev2md -c
```

[↑ Table of Contents](#table-of-contents)

---

## Options

| Option | Description |
|--------|-------------|
| `-o output_dir` | Write output files to this directory. Default: `output/` under current directory. |
| `-r, --reset` | Clear the output directory before converting. Without `-r`, existing files remain and only converted files are overwritten. |
| `-c, --clean` | Interactive cleanup for generated working directories/files. |
| `-h, --help` | Show help. |

[↑ Table of Contents](#table-of-contents)

---

## Processing Pipeline

### ENEX input

1. Parse ENEX XML → extract `<note>` elements  
2. Decode base64 `<resource>` data (images) → build MD5 hash map  
3. Replace `<en-media hash="...">` with `<img src="images/...">` references  
4. Convert code blocks (`<div style="--en-codeblock:true">` or `<en-codeblock>`)  
5. Run shared Markdown pipeline → write `{title}.md`  

### HTML ZIP input

1. Extract ZIP → find `.html` file  
2. Copy referenced images to `output/images/`  
3. Run shared Markdown pipeline → write `{title}.md`  

### Shared Markdown pipeline

1. Strip `<style>`, `<script>`, `<meta>`, `<link>` tags  
2. Convert HTML → Markdown (ATX headings, GFM tables)  
3. Collapse 3+ blank lines to 2  
4. Demote headings by 1 level (H1→H2, …)  
5. Build TOC from H2/H3 headings  
6. Insert `[contents]` backlinks before each heading  
7. Prepend `# {title}` and `## Contents` with TOC  
8. Fix MDX-incompatible autolinks  
9. Remove escaped chars (`\-` `\*` `\_` `\|`) that render literally in some viewers  

[↑ Table of Contents](#table-of-contents)

---

## Known Pitfalls

### `<en-media>` content loss

The Go HTML5 parser treats `<en-media ... />` as a regular opening tag (not self-closing).  
Content after the tag ends up nested as children and gets dropped on remove/replace.  
**Fix:** `unwrapChildren()` re-parents children as siblings before every remove/replace on `<en-media>` nodes.  

### HTML table conversion

Evernote wraps table cell content in `<div>` tags, which the HTML5 parser moves out of cells (foster parenting).  
**Fix:** `preFlattenTableCells()` strips `<div>` wrappers adjacent to `<th>`/`<td>` in the raw HTML string before parsing.  
Missing `| --- |` separator rows are inserted by `addTableSeparators()` after Markdown conversion.  

### Markdown escape sequences

The `html-to-markdown` library escapes `\-` `\*` `\_` `\|` to prevent misinterpretation.  
Some viewers (e.g. ServiceNow KB) display these literally instead of stripping the backslash.  
**Fix:** Escape sequences are removed in a final pass.  

[↑ Table of Contents](#table-of-contents)

---

## Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/JohannesKaufmann/html-to-markdown v1.6.0` | HTML → Markdown conversion (ATX headings, GFM tables) |
| `golang.org/x/net` | `html.Parse` for parsing HTML/ENML |
| stdlib: `archive/zip`, `encoding/xml`, `encoding/base64`, `crypto/md5` | ZIP, XML, ENEX image handling |

[↑ Table of Contents](#table-of-contents)
