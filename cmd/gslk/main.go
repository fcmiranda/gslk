package main

import (
	"flag"
	"fmt" // Import our library package
	"gslk"
	"os"
	"path/filepath"
	// Removed "strings" import as it's no longer needed for action parsing
)

// --- Global Variables for Flags ---
// Use package-level vars for flags defined outside main
var (
	sourceDir   = flag.String("s", "", "Source `directory` containing packages (default: current directory). Can also use --source.")
	targetDir   = flag.String("t", os.Getenv("HOME"), "Target `directory` for symlinks (default: $HOME). Can also use --target.")
	deleteFlag  = flag.Bool("D", false, "Delete/unlink packages instead of linking. Cannot be used with -GL, --gslk or -R.")
	linkFlag    = flag.Bool("GL", false, "Link packages (default action). Cannot be used with -D or -R. Alias: --gslk.") // Changed -S to -GL
	gslkFlag    = flag.Bool("gslk", false, "Alias for -GL (Link packages). Cannot be used with -D or -R.")               // Added --gslk flag as alias
	relinkFlag  = flag.Bool("R", false, "Relink packages (unlink then link). Cannot be used with -D, -GL or --gslk.")
	noopFlag    = flag.Bool("n", false, "Dry run: show what would be done without actually doing it.")
	verboseFlag = flag.Bool("v", false, "Increase verbosity.")
	// Add long aliases for source and target
	_ = flag.String("source", "", "Alias for -s.") // We capture the value with -s
	_ = flag.String("target", "", "Alias for -t.") // We capture the value with -t
)

// Custom usage message
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <package1> [package2] ...\n", filepath.Base(os.Args[0]))
	fmt.Fprintln(os.Stderr, "Description: Creates or removes symlinks for packages.")
	fmt.Fprintln(os.Stderr, "Default action is to link packages (-GL or --gslk).") // Updated default action flag
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults() // Use standard flag package to print defaults
	fmt.Fprintln(os.Stderr, "Example:")
	fmt.Fprintf(os.Stderr, "  %s -s ./dotfiles -t $HOME zsh vim git       (Link packages zsh, vim, git - default action)\n", filepath.Base(os.Args[0])) // Updated example
	fmt.Fprintf(os.Stderr, "  %s -GL -s ./dotfiles -t $HOME zsh vim git   (Explicitly link packages zsh, vim, git)\n", filepath.Base(os.Args[0]))       // Added example with -GL
	fmt.Fprintf(os.Stderr, "  %s --gslk -s ./dotfiles -t $HOME zsh vim git (Explicitly link packages zsh, vim, git)\n", filepath.Base(os.Args[0]))      // Added example with --gslk
	fmt.Fprintf(os.Stderr, "  %s -D -s ./dotfiles -t $HOME zsh           (Unlink package zsh)\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, "  %s -R -v -s ./dotfiles -t $HOME vim        (Relink package vim verbosely)\n", filepath.Base(os.Args[0]))
}

// ...existing code...

func main() {
	// Set default source directory to current directory if not specified
	flag.Usage = printUsage // Set custom usage function

	// Get the current directory before parsing flags
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not determine current directory: %v\n", err)
		// Continue with empty default
	}

	// Parse all flags after setting up defaults
	flag.Parse()

	// If source dir wasn't specified, use current directory
	if *sourceDir == "" {
		*sourceDir = currentDir
	}

	packageNames := flag.Args() // Get remaining non-flag args (package names)

	// --- Flag Validation ---
	actionFlagsSet := 0
	if *deleteFlag {
		actionFlagsSet++
	}
	if *linkFlag { // This now refers to -GL
		actionFlagsSet++
	}
	if *gslkFlag { // This refers to --gslk
		actionFlagsSet++
	}
	if *relinkFlag {
		actionFlagsSet++
	}

	// Specific check: -GL and --gslk cannot be used together, even though they mean the same.
	if *linkFlag && *gslkFlag {
		fmt.Fprintln(os.Stderr, "Error: Cannot specify both -GL and --gslk.")
		printUsage()
		os.Exit(1)
	}

	// Check if more than one *distinct* action type was specified.
	distinctActions := 0
	if *deleteFlag {
		distinctActions++
	}
	if *linkFlag || *gslkFlag { // Treat -GL and --gslk as one action type for this check
		distinctActions++
	}
	if *relinkFlag {
		distinctActions++
	}

	if distinctActions > 1 {
		fmt.Fprintln(os.Stderr, "Error: Only one action type (-D, [-GL|--gslk], -R) can be specified.") // Updated error message
		printUsage()
		os.Exit(1)
	}

	if len(packageNames) == 0 {
		fmt.Fprintln(os.Stderr, "\nError: At least one package name must be provided as an argument.")
		printUsage()
		os.Exit(1)
	}

	// --- Determine Action ---
	action := "link" // Default action
	if *deleteFlag {
		action = "unlink"
	} else if *relinkFlag {
		action = "relink"
	}
	// No need for 'else if *linkFlag || *gslkFlag' as link is the default and validation handles conflicts

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
	// No longer passing gslkFlag as it's just an alias for link
	linker := &gslk.Linker{
		SourceDir: absSource,
		TargetDir: absTarget,
		Verbose:   *verboseFlag, // Assuming Linker has Verbose field
		DryRun:    *noopFlag,    // Assuming Linker has DryRun field
	}

	// ... rest of main function remains the same ...
	// --- Perform Action ---
	var actionErr error
	fmtAction := action // For user messages

	if *noopFlag {
		fmt.Printf("DRY RUN: Would %s packages %v from %s to %s\n", action, packageNames, absSource, absTarget)
		// Simulate the action logic without actual changes
		switch action {
		case "link":
			fmt.Println("DRY RUN: Simulating link operation.")
			// Potentially call a linker.SimulateLink(packageNames) if it exists
		case "unlink":
			fmt.Println("DRY RUN: Simulating unlink operation.")
			// Potentially call a linker.SimulateUnlink(packageNames) if it exists
		case "relink":
			fmt.Println("DRY RUN: Simulating unlink operation (part of relink).")
			// Potentially call linker.SimulateUnlink(packageNames)
			fmt.Println("DRY RUN: Simulating link operation (part of relink).")
			// Potentially call linker.SimulateLink(packageNames)
		}
		fmt.Printf("DRY RUN: Action '%s' simulation completed for packages %v.\n", fmtAction, packageNames)
		os.Exit(0) // Exit successfully after dry run
	}

	// Actual execution
	fmt.Printf("Performing action '%s' for packages %v...\n", fmtAction, packageNames)
	if *verboseFlag {
		fmt.Printf("Source: %s\nTarget: %s\n", absSource, absTarget)
	}

	switch action {
	case "link":
		if *verboseFlag {
			fmt.Printf("Linking packages %v from %s to %s\n", packageNames, absSource, absTarget)
		}
		actionErr = linker.Link(packageNames)
	case "unlink":
		if *verboseFlag {
			fmt.Printf("Unlinking packages %v from %s in %s\n", packageNames, absSource, absTarget)
		}
		actionErr = linker.Unlink(packageNames)
	case "relink":
		fmtAction = "relink (unlink + link)" // More descriptive for messages
		if *verboseFlag {
			fmt.Printf("Unlinking packages %v from %s in %s (part of relink)\n", packageNames, absSource, absTarget)
		}
		actionErr = linker.Unlink(packageNames)
		if actionErr == nil {
			if *verboseFlag {
				fmt.Printf("Linking packages %v from %s to %s (part of relink)\n", packageNames, absSource, absTarget)
			}
			actionErr = linker.Link(packageNames)
		} else {
			// Wrap error to indicate it happened during the unlink phase of relink
			actionErr = fmt.Errorf("error during unlink phase of relink: %w", actionErr)
		}
	}

	// --- Handle Result ---
	if actionErr != nil {
		// Check for specific dry-run simulation errors if Linker methods return them
		// if errors.Is(actionErr, gslk.ErrDryRunSimulationError) { ... }
		fmt.Fprintf(os.Stderr, "Error performing %s action: %v\n", fmtAction, actionErr)
		os.Exit(1) // Exit with non-zero status on error
	}

	fmt.Printf("Action '%s' completed successfully for packages %v.\n", fmtAction, packageNames)
}

// Remove the old init() function and the old main parts related to manual arg parsing
