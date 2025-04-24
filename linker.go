package gslk

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Package represents a directory containing files/folders to be linked.
type Package struct {
	Name string
	Path string
}

// Linker manages the process of linking and unlinking packages.
type Linker struct {
	SourceDir   string
	TargetDir   string
	Verbose     bool
	DryRun      bool
	ForceRemove bool // If true, force-remove parent directories even if not empty
}

// logVerbose logs a message if verbose mode is enabled
func (l *Linker) logVerbose(format string, args ...interface{}) {
	if l.Verbose {
		fmt.Printf(format, args...)
	}
}

// FindPackages discovers packages (subdirectories) within the source directory.
func (l *Linker) FindPackages() ([]Package, error) {
	entries, err := os.ReadDir(l.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read source directory %s: %w", l.SourceDir, err)
	}

	var packages []Package
	for _, entry := range entries {
		if entry.IsDir() {
			// Assuming every directory directly under SourceDir is a package
			packageName := entry.Name()
			packagePath := filepath.Join(l.SourceDir, packageName)
			packages = append(packages, Package{Name: packageName, Path: packagePath})
		}
	}

	if len(packages) == 0 {
		return nil, fmt.Errorf("no packages found in source directory %s", l.SourceDir)
	}

	return packages, nil
}

// loadIgnorePatterns reads the .gslk-ignore file from the given package directory
// and returns a list of ignore patterns. Returns an empty list if the file doesn't exist.
func loadIgnorePatterns(packagePath string) ([]string, error) {
	ignoreFilePath := filepath.Join(packagePath, ".gslk-ignore")
	file, err := os.Open(ignoreFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // No ignore file, return empty list
		}
		return nil, fmt.Errorf("failed to open ignore file %s: %w", ignoreFilePath, err)
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Ignore empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ignore file %s: %w", ignoreFilePath, err)
	}

	return patterns, nil
}

// isPathIgnored checks if a path should be ignored based on the provided patterns
func isPathIgnored(relPath string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		// Check against the full relative path first
		matched, matchErr := filepath.Match(pattern, relPath)
		if matchErr != nil {
			// Log or handle bad patterns
			fmt.Printf("Warning: Invalid pattern '%s': %v\n", pattern, matchErr)
			continue
		}

		// If not matched and pattern doesn't contain a separator, try matching basename
		if !matched && !strings.Contains(pattern, string(filepath.Separator)) {
			baseName := filepath.Base(relPath)
			matched, matchErr = filepath.Match(pattern, baseName)
			if matchErr != nil {
				fmt.Printf("Warning: Error matching pattern '%s' against base name '%s': %v\n", pattern, baseName, matchErr)
				continue
			}
		}

		if matched {
			return true
		}
	}
	return false
}

// removeParents attempts to remove the parent directory of targetPath
// and continues removing parent directories upwards until
// it hits the baseDir, root, or outside base.
// If force is true, directories will be removed even if they're not empty.
func removeParents(targetPath string, baseDir string, force bool) {
	parentDir := filepath.Dir(targetPath)
	// Ensure baseDir is absolute for reliable comparison
	absBaseDir, err := filepath.Abs(baseDir)
	if err != nil {
		fmt.Printf("Warning: could not get absolute path for baseDir %s: %v\n", baseDir, err)
		absBaseDir = baseDir // Proceed with potentially relative path
	}

	for {
		absParentDir, err := filepath.Abs(parentDir)
		if err != nil {
			fmt.Printf("Warning: could not get absolute path for parentDir %s: %v\n", parentDir, err)
			break // Cannot reliably compare, stop
		}

		// Stop conditions: reached base, root, or outside base
		// Use filepath.Clean to handle trailing slashes etc.
		if filepath.Clean(absParentDir) == filepath.Clean(absBaseDir) || absParentDir == "/" || absParentDir == "." || !strings.HasPrefix(absParentDir, absBaseDir) {
			break
		}

		// Attempt to remove the directory
		var removeErr error
		if force {
			// Force remove the directory and all its contents
			removeErr = os.RemoveAll(parentDir)
		} else {
			// Only remove if empty (default behavior)
			removeErr = os.Remove(parentDir)
		}

		if removeErr == nil {
			fmt.Printf("Removed directory: %s\n", parentDir)
			// Move up to the next parent
			parentDir = filepath.Dir(parentDir)
		} else {
			// Log the failure reason if verbose
			if force {
				fmt.Printf("Failed to force-remove directory %s: %v\n", parentDir, removeErr)
			} else {
				// Likely not empty, which is expected behavior
				fmt.Printf("Skipped non-empty directory: %s\n", parentDir)
			}
			break
		}
	}
}

// processPackagePaths walks the package directory and returns a list of file paths to process
// along with their corresponding target paths and relative paths
type pathInfo struct {
	sourcePath string
	targetPath string
	relPath    string
	isDir      bool
}

func (l *Linker) processPackagePaths(pkg Package, ignorePatterns []string) ([]pathInfo, error) {
	var paths []pathInfo

	err := filepath.WalkDir(pkg.Path, func(sourcePath string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("error accessing %s: %w", sourcePath, walkErr)
		}

		// Skip the root package directory itself and the ignore file
		if sourcePath == pkg.Path || filepath.Base(sourcePath) == ".gslk-ignore" {
			return nil
		}

		relPath, err := filepath.Rel(pkg.Path, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", sourcePath, err)
		}

		// Check against ignore patterns
		if isPathIgnored(relPath, ignorePatterns) {
			l.logVerbose("Ignoring %s (matches ignore pattern)\n", relPath)
			if d.IsDir() {
				return filepath.SkipDir // Skip the entire directory
			}
			return nil // Skip this file
		}

		targetPath := filepath.Join(l.TargetDir, relPath)

		paths = append(paths, pathInfo{
			sourcePath: sourcePath,
			targetPath: targetPath,
			relPath:    relPath,
			isDir:      d.IsDir(),
		})

		return nil
	})

	return paths, err
}

// ensureDirectory creates a directory if it doesn't exist
func (l *Linker) ensureDirectory(path string) error {
	if l.DryRun {
		l.logVerbose("DRY RUN: Would create directory: %s\n", path)
		return nil
	}

	l.logVerbose("Ensuring directory exists: %s\n", path)
	return os.MkdirAll(path, 0755)
}

// createSymlink creates a symbolic link from target to source
func (l *Linker) createSymlink(sourcePath, targetPath string) error {
	fmt.Printf("Linking: %s -> %s\n", sourcePath, targetPath)

	if l.DryRun {
		return nil
	}

	// Ensure parent directory exists
	targetDir := filepath.Dir(targetPath)
	if err := l.ensureDirectory(targetDir); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
	}

	// Create the symbolic link with absolute path
	absSourcePath, absErr := filepath.Abs(sourcePath)
	if absErr != nil {
		return fmt.Errorf("failed to get absolute path for source %s: %w", sourcePath, absErr)
	}

	return os.Symlink(absSourcePath, targetPath)
}

// isCorrectSymlink checks if a symlink at targetPath correctly points to sourcePath
func isCorrectSymlink(targetPath, sourcePath string) (bool, error) {
	linkTarget, err := os.Readlink(targetPath)
	if err != nil {
		return false, fmt.Errorf("failed to read symlink %s: %w", targetPath, err)
	}

	// Compare absolute paths for robustness
	absSourcePath, err := filepath.Abs(sourcePath)
	if err != nil {
		return false, fmt.Errorf("failed to get absolute path for source %s: %w", sourcePath, err)
	}

	absLinkTarget, err := filepath.Abs(filepath.Join(filepath.Dir(targetPath), linkTarget))
	if err != nil {
		// Try absolute if linkTarget itself was absolute
		absLinkTarget, err = filepath.Abs(linkTarget)
		if err != nil {
			return false, fmt.Errorf("failed to get absolute path for link target %s: %w", linkTarget, err)
		}
	}

	return linkTarget == sourcePath || absLinkTarget == absSourcePath, nil
}

// Link creates symbolic links for the specified packages from SourceDir to TargetDir.
// It handles conflicts if a file/directory already exists at the target location.
func (l *Linker) Link(packageNames []string) error {
	allPackages, err := l.FindPackages()
	if err != nil {
		return fmt.Errorf("failed to find packages: %w", err)
	}

	packagesToLink := make(map[string]Package)
	for _, pkg := range allPackages {
		packagesToLink[pkg.Name] = pkg
	}

	for _, name := range packageNames {
		pkg, ok := packagesToLink[name]
		if !ok {
			return fmt.Errorf("package '%s' not found in source directory %s", name, l.SourceDir)
		}

		// Load ignore patterns for this package
		ignorePatterns, err := loadIgnorePatterns(pkg.Path)
		if err != nil {
			return fmt.Errorf("failed to load ignore patterns for package %s: %w", name, err)
		}

		l.logVerbose("Loaded %d ignore patterns for package %s\n", len(ignorePatterns), name)

		// Process all paths in the package
		paths, err := l.processPackagePaths(pkg, ignorePatterns)
		if err != nil {
			return fmt.Errorf("failed to process paths for package %s: %w", name, err)
		}

		// Handle each path
		for _, path := range paths {
			if path.isDir {
				// For directories, just ensure they exist in target
				if err := l.ensureDirectory(path.targetPath); err != nil {
					return fmt.Errorf("failed to create target directory %s: %w", path.targetPath, err)
				}
				continue
			}

			// For files, check if target already exists
			targetFi, err := os.Lstat(path.targetPath)
			if err == nil {
				// Target exists, check if it's a symlink to the correct source
				if targetFi.Mode()&os.ModeSymlink != 0 {
					isCorrect, checkErr := isCorrectSymlink(path.targetPath, path.sourcePath)
					if checkErr != nil {
						return checkErr
					}

					if isCorrect {
						// Already correctly linked, skip
						l.logVerbose("Skipping already linked: %s -> %s\n", path.sourcePath, path.targetPath)
						continue
					}
				}
				// Target exists but is not the correct symlink
				return fmt.Errorf("conflict: target %s already exists and is not the expected symlink", path.targetPath)
			} else if !os.IsNotExist(err) {
				// Error during Lstat other than file not existing
				return fmt.Errorf("failed to stat target path %s: %w", path.targetPath, err)
			}

			// Create symlink
			if err := l.createSymlink(path.sourcePath, path.targetPath); err != nil {
				return fmt.Errorf("failed to create symlink from %s to %s: %w", path.sourcePath, path.targetPath, err)
			}
		}
	}

	return nil
}

// Unlink removes symbolic links for the specified packages from the TargetDir
// that point back to the SourceDir. It also removes empty parent directories
// created during linking.
func (l *Linker) Unlink(packageNames []string) error {
	allPackages, err := l.FindPackages()
	if err != nil {
		return fmt.Errorf("failed to find packages: %w", err)
	}

	packagesToUnlink := make(map[string]Package)
	for _, pkg := range allPackages {
		packagesToUnlink[pkg.Name] = pkg
	}

	for _, name := range packageNames {
		pkg, ok := packagesToUnlink[name]
		if !ok {
			return fmt.Errorf("package '%s' not found in source directory %s, cannot determine links to remove", name, l.SourceDir)
		}

		// Load ignore patterns for this package
		ignorePatterns, err := loadIgnorePatterns(pkg.Path)
		if err != nil {
			return fmt.Errorf("failed to load ignore patterns for package %s: %w", name, err)
		}

		l.logVerbose("Loaded %d ignore patterns for package %s for unlinking\n", len(ignorePatterns), name)

		// Process all paths in the package
		paths, err := l.processPackagePaths(pkg, ignorePatterns)
		if err != nil {
			return fmt.Errorf("failed to process paths for package %s: %w", name, err)
		}

		// Handle each path that is not a directory
		for _, path := range paths {
			if path.isDir {
				continue // Skip directories during unlinking
			}

			targetFi, err := os.Lstat(path.targetPath)
			if err != nil {
				if os.IsNotExist(err) {
					// Target doesn't exist, nothing to unlink
					continue
				}
				// Other error stat-ing target
				return fmt.Errorf("failed to stat target path %s: %w", path.targetPath, err)
			}

			// Target exists, check if it's a symlink pointing to our source
			if targetFi.Mode()&os.ModeSymlink != 0 {
				isCorrect, checkErr := isCorrectSymlink(path.targetPath, path.sourcePath)
				if checkErr != nil {
					return checkErr
				}

				if isCorrect {
					// This is the link we created, remove it
					fmt.Printf("Unlinking: %s (link to %s)\n", path.targetPath, path.sourcePath)

					// In dry run mode, don't make actual changes
					if l.DryRun {
						continue
					}

					removeErr := os.Remove(path.targetPath)
					if removeErr != nil && !os.IsNotExist(removeErr) {
						return fmt.Errorf("failed to remove symlink %s: %w", path.targetPath, removeErr)
					}

					// Attempt to remove empty parent directories
					removeParents(path.targetPath, l.TargetDir, l.ForceRemove)
				} else if l.Verbose {
					// Symlink exists but points elsewhere
					fmt.Printf("Skipping unlink for %s: symlink points elsewhere\n", path.targetPath)
				}
			} else if l.Verbose {
				// Target exists but is not a symlink
				fmt.Printf("Skipping unlink for %s: not a symlink\n", path.targetPath)
			}
		}
	}

	// Verification pass if not in dry run mode
	if !l.DryRun {
		err = l.verifyUnlink(packageNames, packagesToUnlink)
		if err != nil {
			return err
		}
	}

	return nil
}

// verifyUnlink performs a verification pass to ensure no lingering links exist
func (l *Linker) verifyUnlink(packageNames []string, packagesToUnlink map[string]Package) error {
	for _, name := range packageNames {
		pkg, ok := packagesToUnlink[name]
		if !ok {
			continue // We've already checked this earlier
		}

		// Load ignore patterns again for verification
		ignorePatterns, err := loadIgnorePatterns(pkg.Path)
		if err != nil {
			return fmt.Errorf("failed to load ignore patterns for package %s during verification: %w", name, err)
		}

		// Process all paths for verification
		paths, err := l.processPackagePaths(pkg, ignorePatterns)
		if err != nil {
			return fmt.Errorf("failed to process paths for package %s during verification: %w", name, err)
		}

		// Check each file (not directory)
		for _, path := range paths {
			if !path.isDir {
				targetFi, err := os.Lstat(path.targetPath)
				if err == nil && targetFi.Mode()&os.ModeSymlink != 0 {
					// Link still exists, check if it points to our source
					isCorrect, _ := isCorrectSymlink(path.targetPath, path.sourcePath)
					if isCorrect {
						return fmt.Errorf("symbolic link %s still exists after unlink operation", path.targetPath)
					}
				}
			}
		}
	}
	return nil
}
