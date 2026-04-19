# ev2md

Convert Evernote exports (ENEX or HTML ZIP) into Markdown documents.  

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Usage](#usage)
- [Options](#options)

---

## Overview

`ev2md` converts Evernote ENEX exports or Evernote HTML ZIP exports into Markdown files with properly managed image assets.  
Output is `.md` files and an `images/` directory written to the specified output directory.  
Single file and batch (directory) conversion are both supported.  

Suggestions: open an issue at https://github.com/junsulee75/ev2md/issues

[↑ Table of Contents](#table-of-contents)

---

## Installation

### Install via script

```bash
curl -s https://raw.githubusercontent.com/junsulee75/ev2md/main/install.sh | bash
```

Detects OS and architecture automatically and installs to `~/bin/ev2md`.  

### Build from source

#### Prerequisites

- Go 1.21 or later: https://go.dev/dl/

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
