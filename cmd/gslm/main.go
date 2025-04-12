package main

import (
	"flag"
	"fmt"
	"glsm" // Import our library package
	"os"
	"path/filepath"
	"strings"
)

// Custom usage message (moved before main for clarity)
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <action> [options] <package1> [package2] ...\n", filepath.Base(os.Args[0]))
	fmt.Fprintln(os.Stderr, "Actions:")
	fmt.Fprintln(os.Stderr, "  link      Create symlinks for packages")
	fmt.Fprintln(os.Stderr, "  unlink    Remove symlinks for packages")
	fmt.Fprintln(os.Stderr, "Options:")
	// Use a custom FlagSet to print defaults correctly with our usage message
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.String("s", "", "Source directory containing packages (required)")
	fs.String("t", "", "Target directory for symlinks (required)")
	fs.SetOutput(os.Stderr) // Ensure defaults print to stderr
	fs.PrintDefaults()
	fmt.Fprintln(os.Stderr, "Example:")
	fmt.Fprintf(os.Stderr, "  %s link -s ./dotfiles -t $HOME zsh vim git\n", filepath.Base(os.Args[0]))
}

func main() {
	// Define flags with short options using a custom FlagSet
	// This is needed to parse flags correctly after the action argument
	// and to integrate with our custom printUsage
	cliFlags := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cliFlags.Usage = printUsage // Set custom usage function

	sourceDir := cliFlags.String("s", "", "Source directory containing packages (required)")
	targetDir := cliFlags.String("t", "", "Target directory for symlinks (required)")

	// --- Argument Parsing & Validation ---
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Error: Action (link or unlink) is required.")
		printUsage()
		os.Exit(1)
	}

	// Check for help flag *before* parsing other flags
	for _, arg := range os.Args {
		if arg == "-h" || arg == "--help" {
			printUsage()
			os.Exit(0)
		}
	}

	action := strings.ToLower(os.Args[1])
	if action != "link" && action != "unlink" {
		fmt.Fprintf(os.Stderr, "Error: Invalid action '%s'. Must be 'link' or 'unlink'.\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	// Parse flags from arguments *after* the action
	// Use the custom FlagSet cliFlags
	err := cliFlags.Parse(os.Args[2:]) // Parse flags from the rest of the args
	if err != nil {
		// Error handled by cliFlags.Usage or flag.ExitOnError
		return
	}
	packageNames := cliFlags.Args() // Get remaining non-flag args (package names)

	// --- Flag Validation ---
	if *sourceDir == "" || *targetDir == "" {
		fmt.Fprintln(os.Stderr, "\nError: -s (source) and -t (target) flags are required.")
		printUsage()
		os.Exit(1)
	}

	if len(packageNames) == 0 {
		fmt.Fprintln(os.Stderr, "\nError: At least one package name must be provided as an argument.")
		printUsage()
		os.Exit(1)
	}

	// Resolve paths to absolute for consistency
	absSource, err := filepath.Abs(*sourceDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving source directory path %s: %v\n", *sourceDir, err)
		os.Exit(1)
	}
	absTarget, err := filepath.Abs(*targetDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving target directory path %s: %v\n", *targetDir, err)
		os.Exit(1)
	}

	// --- Initialize Linker ---
	linker := &glsm.Linker{
		SourceDir: absSource,
		TargetDir: absTarget,
	}

	// --- Perform Action ---
	var actionErr error
	switch action { // Use the parsed action variable
	case "link":
		fmt.Printf("Linking packages %v from %s to %s\n", packageNames, absSource, absTarget)
		actionErr = linker.Link(packageNames)
	case "unlink":
		fmt.Printf("Unlinking packages %v from %s in %s\n", packageNames, absSource, absTarget)
		actionErr = linker.Unlink(packageNames)
		// No default needed as we validated the action earlier
	}

	// --- Handle Result ---
	if actionErr != nil {
		fmt.Fprintf(os.Stderr, "Error performing %s action: %v\n", action, actionErr)
		os.Exit(1) // Exit with non-zero status on error
	}

	fmt.Printf("Action '%s' completed successfully for packages %v.\n", action, packageNames)
}

// Remove the old init() function that set flag.Usage
/*
func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <package1> [package2] ...\n", filepath.Base(os.Args[0]))
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "Example:")
		fmt.Fprintf(os.Stderr, "  %s --source ./dotfiles --target $HOME zsh vim git\n", filepath.Base(os.Args[0]))
	}
}
*/
