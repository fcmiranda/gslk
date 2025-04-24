package main

import (
	"flag"
	"fmt"
	"gslk"
	"os"
	"path/filepath"
	"strings"
)

// Action constants
const (
	actionLink   = "link"
	actionUnlink = "unlink"
	actionRelink = "relink"
)

// Flags
var (
	sourceDir       = flag.String("s", "", "Source `directory` containing packages (default: current directory). Can also use --source.")
	targetDir       = flag.String("t", os.Getenv("HOME"), "Target `directory` for symlinks (default: $HOME). Can also use --target.")
	deleteFlag      = flag.Bool("D", false, "Delete/unlink packages instead of linking. Cannot be used with -GL, --gslk or -R.")
	linkFlag        = flag.Bool("GL", false, "Link packages (default action). Cannot be used with -D or -R. Alias: --gslk.")
	gslkFlag        = flag.Bool("gslk", false, "Alias for -GL (Link packages). Cannot be used with -D or -R.")
	relinkFlag      = flag.Bool("R", false, "Relink packages (unlink then link). Cannot be used with -D, -GL or --gslk.")
	noopFlag        = flag.Bool("n", false, "Dry run: show what would be done without actually doing it.")
	verboseFlag     = flag.Bool("v", false, "Increase verbosity.")
	forceRemoveFlag = flag.Bool("f", false, "Force remove parent directories during unlink, even if not empty.")
	_               = flag.String("source", "", "Alias for -s.")
	_               = flag.String("target", "", "Alias for -t.")
	_               = flag.Bool("force", false, "Alias for -f.")
)

// printUsage displays the command usage information
func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] <package1> [package2] ...\n", filepath.Base(os.Args[0]))
	fmt.Fprintln(os.Stderr, "Description: Creates or removes symlinks for packages.")
	fmt.Fprintln(os.Stderr, "Default action is to link packages (-GL or --gslk).")
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
	fmt.Fprintln(os.Stderr, "Example:")
	fmt.Fprintf(os.Stderr, "  %s -s ./dotfiles -t $HOME zsh vim git       (Link packages zsh, vim, git - default action)\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, "  %s -GL -s ./dotfiles -t $HOME zsh vim git   (Explicitly link packages zsh, vim, git)\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, "  %s --gslk -s ./dotfiles -t $HOME zsh vim git (Explicitly link packages zsh, vim, git)\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, "  %s -D -s ./dotfiles -t $HOME zsh           (Unlink package zsh with verification)\n", filepath.Base(os.Args[0]))
	fmt.Fprintf(os.Stderr, "  %s -R -v -s ./dotfiles -t $HOME vim        (Relink package vim verbosely)\n", filepath.Base(os.Args[0]))
}

// validateFlags checks for flag conflicts and proper usage
func validateFlags(packageNames []string) (string, error) {
	// Check for misinterpreted flags in packageNames
	for _, name := range packageNames {
		if strings.HasPrefix(name, "-") {
			return "", fmt.Errorf("'%s' looks like a flag but was interpreted as a package name. All flags must come before package names", name)
		}
	}

	// Check for package names
	if len(packageNames) == 0 {
		return "", fmt.Errorf("at least one package name must be provided as an argument")
	}

	// Specific check: -GL and --gslk cannot be used together
	if *linkFlag && *gslkFlag {
		return "", fmt.Errorf("cannot specify both -GL and --gslk")
	}

	// Check for conflicting action flags
	distinctActions := 0
	if *deleteFlag {
		distinctActions++
	}
	if *linkFlag || *gslkFlag {
		distinctActions++
	}
	if *relinkFlag {
		distinctActions++
	}

	if distinctActions > 1 {
		return "", fmt.Errorf("only one action type (-D, [-GL|--gslk], -R) can be specified")
	}

	// Determine action
	action := actionLink // Default action
	if *deleteFlag {
		action = actionUnlink
	} else if *relinkFlag {
		action = actionRelink
	}

	return action, nil
}

// setupLinker creates and configures the gslk.Linker instance
func setupLinker() (*gslk.Linker, error) {
	// Get current directory for default source
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("could not determine current directory: %v", err)
	}

	// If source dir wasn't specified, use current directory
	sourceDirectory := *sourceDir
	if sourceDirectory == "" {
		sourceDirectory = currentDir
	}

	// Resolve paths to absolute for consistency
	absSource, err := filepath.Abs(sourceDirectory)
	if err != nil {
		return nil, fmt.Errorf("error resolving source directory path %s: %v", sourceDirectory, err)
	}

	absTarget, err := filepath.Abs(*targetDir)
	if err != nil {
		return nil, fmt.Errorf("error resolving target directory path %s: %v", *targetDir, err)
	}

	return &gslk.Linker{
		SourceDir:   absSource,
		TargetDir:   absTarget,
		Verbose:     *verboseFlag,
		DryRun:      *noopFlag,
		ForceRemove: *forceRemoveFlag,
	}, nil
}

// performAction executes the specified action
func performAction(linker *gslk.Linker, action string, packageNames []string) error {
	if *verboseFlag {
		fmt.Printf("Source: %s\nTarget: %s\n", linker.SourceDir, linker.TargetDir)
	}

	switch action {
	case actionLink:
		if *verboseFlag {
			fmt.Printf("Linking packages %v from %s to %s\n", packageNames, linker.SourceDir, linker.TargetDir)
		}
		return linker.Link(packageNames)

	case actionUnlink:
		if *verboseFlag {
			fmt.Printf("Unlinking packages %v from %s in %s\n", packageNames, linker.SourceDir, linker.TargetDir)
			fmt.Println("Verification will ensure all symbolic links are properly removed")
		}
		return linker.Unlink(packageNames)

	case actionRelink:
		if *verboseFlag {
			fmt.Printf("Unlinking packages %v from %s in %s (part of relink)\n", packageNames, linker.SourceDir, linker.TargetDir)
		}

		err := linker.Unlink(packageNames)
		if err != nil {
			return fmt.Errorf("error during unlink phase of relink: %w", err)
		}

		if *verboseFlag {
			fmt.Printf("Linking packages %v from %s to %s (part of relink)\n", packageNames, linker.SourceDir, linker.TargetDir)
		}
		return linker.Link(packageNames)

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}

// simulateAction performs a dry run of the specified action
func simulateAction(linker *gslk.Linker, action string, packageNames []string) {
	fmt.Printf("DRY RUN: Would %s packages %v from %s to %s\n", action, packageNames, linker.SourceDir, linker.TargetDir)

	switch action {
	case actionLink:
		fmt.Println("DRY RUN: Simulating link operation.")
	case actionUnlink:
		fmt.Println("DRY RUN: Simulating unlink operation.")
	case actionRelink:
		fmt.Println("DRY RUN: Simulating unlink operation (part of relink).")
		fmt.Println("DRY RUN: Simulating link operation (part of relink).")
	}

	fmt.Printf("DRY RUN: Action '%s' simulation completed for packages %v.\n", action, packageNames)
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	packageNames := flag.Args()

	// Validate flags and determine action
	action, err := validateFlags(packageNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "")
		printUsage()
		os.Exit(1)
	}

	// Setup linker
	linker, err := setupLinker()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Handle dry run mode
	if *noopFlag {
		simulateAction(linker, action, packageNames)
		os.Exit(0)
	}

	// Perform the actual action
	fmt.Printf("Performing action '%s' for packages %v...\n", action, packageNames)

	err = performAction(linker, action, packageNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error performing %s action: %v\n", action, err)
		os.Exit(1)
	}

	fmt.Printf("Action '%s' completed successfully for packages %v.\n", action, packageNames)
}
