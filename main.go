package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var buildDate = "dev"

func main() {
	inputFiles, outputFile, cleanMode, cpMode, resetMode, cpArgs := parseArgs()

	if cleanMode {
		interactiveClean()
		os.Exit(0)
	}

	if cpMode {
		var err error
		switch len(cpArgs) {
		case 0:
			err = copyAndExtract("", "")
		case 2:
			err = copyAndExtract(cpArgs[0], cpArgs[1])
		default:
			printUsage()
			os.Exit(1)
		}
		if err != nil {
			fmt.Println("[ERROR]", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if len(inputFiles) == 0 {
		printUsage()
		os.Exit(1)
	}

	outputDir := outputFile
	if outputDir == "" {
		outputDir = "output"
	}

	// Collect inputs: if a single directory is given, scan it; otherwise use args directly
	var inputs []string
	if len(inputFiles) == 1 {
		info, err := os.Stat(inputFiles[0])
		if err != nil {
			fmt.Printf("Error: %s not found.\n", inputFiles[0])
			os.Exit(1)
		}
		if info.IsDir() {
			entries, _ := os.ReadDir(inputFiles[0])
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				ext := strings.ToLower(filepath.Ext(e.Name()))
				if ext == ".enex" || ext == ".zip" {
					inputs = append(inputs, filepath.Join(inputFiles[0], e.Name()))
				}
			}
			if len(inputs) == 0 {
				fmt.Printf("No .enex or .zip files found in %s\n", inputFiles[0])
				os.Exit(1)
			}
		} else {
			inputs = inputFiles
		}
	} else {
		inputs = inputFiles
	}

	if resetMode {
		if err := resetDir(outputDir); err != nil {
			fmt.Println("[ERROR]", err)
			os.Exit(1)
		}
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "images"), 0755); err != nil {
		fmt.Println("[ERROR]", err)
		os.Exit(1)
	}

	hasError := false
	for _, inp := range inputs {
		ext := strings.ToLower(filepath.Ext(inp))
		var err error
		switch ext {
		case ".enex":
			err = convertEnex(inp, outputDir)
		case ".zip":
			err = convertEvernoteZip(inp, outputDir)
		}
		if err != nil {
			fmt.Printf("[ERROR] %s: %v\n", inp, err)
			hasError = true
		} else {
			fmt.Printf("  ✅ %s\n", filepath.Base(inp))
		}
	}

	if hasError {
		fmt.Printf("\n⚠️  Done with errors: %s\n", outputDir)
	} else {
		fmt.Printf("\n✅ Done: %s\n", outputDir)
	}
}

// parseArgs parses os.Args manually to support -cp with variable argument count.
func parseArgs() (inputFiles []string, outputFile string, cleanMode, cpMode, resetMode bool, cpArgs []string) {
	args := os.Args[1:]
	i := 0
	for i < len(args) {
		switch args[i] {
		case "-c", "--clean":
			cleanMode = true
			i++
		case "-r", "--reset":
			resetMode = true
			i++
		case "-o", "--output":
			if i+1 < len(args) {
				outputFile = args[i+1]
				i += 2
			} else {
				i++
			}
		case "-cp", "--copy":
			cpMode = true
			i++
			for i < len(args) && !strings.HasPrefix(args[i], "-") {
				cpArgs = append(cpArgs, args[i])
				i++
			}
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			if !strings.HasPrefix(args[i], "-") {
				inputFiles = append(inputFiles, args[i])
			}
			i++
		}
	}
	return
}

func printUsage() {
	fmt.Printf("ev2md  (build: %s)\n", buildDate)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  ev2md <input.enex> [-o output_dir] [-r]")
	fmt.Println("  ev2md <input_dir>  [-o output_dir] [-r]")
	fmt.Println("  ev2md -c")
	fmt.Println("  ev2md -h")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -o output_dir")
	fmt.Println("      Write output files to this directory.")
	fmt.Println("      Default: output/ under the current working directory.")
	fmt.Println()
	fmt.Println("  -r, --reset")
	fmt.Println("      Clear the output directory before converting.")
	fmt.Println("      Without -r: existing files remain; only converted files are overwritten.")
	fmt.Println()
	fmt.Println("  -c, --clean")
	fmt.Println("      Interactive cleanup for generated working directories/files.")
	fmt.Println()
	fmt.Println("  -h, --help")
	fmt.Println("      Show this help.")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Convert a single ENEX file → written to ./output/")
	fmt.Println("  ev2md notes/my_note.enex")
	fmt.Println()
	fmt.Println("  # Convert a single ENEX file to a specific output directory")
	fmt.Println("  ev2md notes/my_note.enex -o /tmp/out")
	fmt.Println()
	fmt.Println("  # Convert all .enex files in a directory → written to ./output/")
	fmt.Println("  ev2md notes/")
	fmt.Println()
	fmt.Println("  # Convert all files in a directory with a specific output path")
	fmt.Println("  ev2md test/ -o test/output")
	fmt.Println()
	fmt.Println("  # Same, but clear output directory first (-r)")
	fmt.Println("  ev2md test/ -o test/output -r")
	fmt.Println()
	fmt.Println("  # Clean generated output/work directories interactively")
	fmt.Println("  ev2md -c")
	fmt.Println()
}

// resetDir removes and recreates a directory.
func resetDir(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0755)
}

