package glsm

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
	SourceDir string
	TargetDir string
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
			// We might add more validation later (e.g., checking for specific files)
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

// loadIgnorePatterns reads the .gslm-ignore file from the given package directory
// and returns a list of ignore patterns. Returns an empty list if the file doesn't exist.
func loadIgnorePatterns(packagePath string) ([]string, error) {
	ignoreFilePath := filepath.Join(packagePath, ".gslm-ignore")
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
			// Option: return error, log warning, or skip?
			// Let's return an error for now.
			return fmt.Errorf("package '%s' not found in source directory %s", name, l.SourceDir)
		}

		// Load ignore patterns for this package
		ignorePatterns, err := loadIgnorePatterns(pkg.Path)
		if err != nil {
			return fmt.Errorf("failed to load ignore patterns for package %s: %w", name, err)
		}
		fmt.Printf("Loaded %d ignore patterns for package %s\n", len(ignorePatterns), name)

		err = filepath.WalkDir(pkg.Path, func(sourcePath string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				// Error accessing the file/directory during walk
				return fmt.Errorf("error accessing %s: %w", sourcePath, walkErr)
			}

			// Skip the root package directory itself and the ignore file
			if sourcePath == pkg.Path || filepath.Base(sourcePath) == ".gslm-ignore" {
				return nil
			}

			relPath, err := filepath.Rel(pkg.Path, sourcePath)
			if err != nil {
				// Should generally not happen if sourcePath is within pkg.Path
				return fmt.Errorf("failed to get relative path for %s: %w", sourcePath, err)
			}

			// Check against ignore patterns
			for _, pattern := range ignorePatterns {
				matched, matchErr := filepath.Match(pattern, relPath)
				if matchErr != nil {
					// Log or handle bad patterns? For now, let's report it.
					fmt.Printf("Warning: Invalid pattern '%s' in .gslm-ignore for package %s: %v\n", pattern, name, matchErr)
					continue // Skip this pattern
				}
				if matched {
					fmt.Printf("Ignoring %s (matches pattern '%s')\n", relPath, pattern)
					if d.IsDir() {
						return filepath.SkipDir // Skip the entire directory
					}
					return nil // Skip this file
				}
			}

			targetPath := filepath.Join(l.TargetDir, relPath)

			// If current item is a directory, ensure it exists in target and continue.
			// Do not create symlinks *for* directories, only *within* them.
			if d.IsDir() {
				fmt.Printf("Ensuring directory exists: %s\n", targetPath)
				if err := os.MkdirAll(targetPath, 0755); err != nil {
					return fmt.Errorf("failed to create target directory %s: %w", targetPath, err)
				}
				return nil // Directory handled, continue walk
			}

			// --- Conflict Detection and Linking Logic (for Files) ---

			// 1. Check if target exists (use os.Lstat to avoid following links)
			// 2. If exists:
			//    a. Check if it's a symlink pointing to sourcePath. If yes, it's okay.
			//    b. Otherwise, it's a conflict. Return an error.
			// 3. If not exists:
			//    a. Ensure target parent directory exists (os.MkdirAll)
			//    b. Create the symlink (os.Symlink)

			// fmt.Printf("Processing: %s -> %s\n", sourcePath, targetPath) // Placeholder
			// TODO: Implement the actual conflict detection and linking

			targetFi, err := os.Lstat(targetPath)
			if err == nil {
				// Target exists, check if it's a symlink to the correct source
				if targetFi.Mode()&os.ModeSymlink != 0 {
					linkTarget, readErr := os.Readlink(targetPath)
					if readErr != nil {
						return fmt.Errorf("failed to read existing symlink %s: %w", targetPath, readErr)
					}
					// Important: Compare absolute paths for robustness
					absSourcePath, absErr := filepath.Abs(sourcePath)
					if absErr != nil {
						return fmt.Errorf("failed to get absolute path for source %s: %w", sourcePath, absErr)
					}
					absLinkTarget, absErr := filepath.Abs(filepath.Join(filepath.Dir(targetPath), linkTarget))
					if absErr != nil {
						// Try absolute if linkTarget itself was absolute
						absLinkTarget, absErr = filepath.Abs(linkTarget)
						if absErr != nil {
							return fmt.Errorf("failed to get absolute path for link target %s: %w", linkTarget, absErr)
						}
					}
					if linkTarget == sourcePath || absLinkTarget == absSourcePath {
						// Already correctly linked, skip
						fmt.Printf("Skipping already linked: %s -> %s\n", sourcePath, targetPath)
						return nil
					}
				}
				// Target exists but is not the correct symlink (or not a symlink at all)
				return fmt.Errorf("conflict: target %s already exists and is not the expected symlink", targetPath)
			} else if !os.IsNotExist(err) {
				// Error during Lstat other than file not existing
				return fmt.Errorf("failed to stat target path %s: %w", targetPath, err)
			}

			// Target does not exist, proceed with linking
			fmt.Printf("Linking: %s -> %s\n", sourcePath, targetPath)

			// Ensure parent directory exists
			targetDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(targetDir, 0755); err != nil {
				return fmt.Errorf("failed to create target directory %s: %w", targetDir, err)
			}

			// Create the symbolic link
			// Use relative path for the link source if possible, otherwise absolute
			// For simplicity now, let's use the absolute source path
			absSourcePath, absErr := filepath.Abs(sourcePath)
			if absErr != nil {
				return fmt.Errorf("failed to get absolute path for source %s: %w", sourcePath, absErr)
			}
			if err := os.Symlink(absSourcePath, targetPath); err != nil {
				return fmt.Errorf("failed to create symlink from %s to %s: %w", absSourcePath, targetPath, err)
			}

			return nil // Continue walking
		})

		if err != nil {
			// Error during walk for this package
			return fmt.Errorf("failed to link package %s: %w", name, err)
		}
	}

	return nil // Success
}

// Unlink removes symbolic links for the specified packages from the TargetDir
// that point back to the SourceDir.
func (l *Linker) Unlink(packageNames []string) error {
	allPackages, err := l.FindPackages()
	if err != nil {
		// Allow unlinking even if source dir has issues, maybe?
		// For now, let's require source to be readable to know *what* to unlink.
		return fmt.Errorf("failed to find packages: %w", err)
	}

	packagesToUnlink := make(map[string]Package)
	for _, pkg := range allPackages {
		packagesToUnlink[pkg.Name] = pkg
	}

	for _, name := range packageNames {
		pkg, ok := packagesToUnlink[name]
		if !ok {
			// If package doesn't exist in source, we can't know what to unlink.
			return fmt.Errorf("package '%s' not found in source directory %s, cannot determine links to remove", name, l.SourceDir)
		}

		// Load ignore patterns for this package
		ignorePatterns, err := loadIgnorePatterns(pkg.Path)
		if err != nil {
			// If we can't load ignores, we might remove files that shouldn't be linked.
			// Let's return an error to be safe.
			return fmt.Errorf("failed to load ignore patterns for package %s: %w", name, err)
		}
		fmt.Printf("Loaded %d ignore patterns for package %s for unlinking\n", len(ignorePatterns), name)

		// We need to walk the source package dir to know what links *should* exist
		err = filepath.WalkDir(pkg.Path, func(sourcePath string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return fmt.Errorf("error accessing %s: %w", sourcePath, walkErr)
			}

			// Skip the root package directory itself and the ignore file
			if sourcePath == pkg.Path || filepath.Base(sourcePath) == ".gslm-ignore" {
				return nil
			}

			relPath, err := filepath.Rel(pkg.Path, sourcePath)
			if err != nil {
				return fmt.Errorf("failed to get relative path for %s: %w", sourcePath, err)
			}

			// Check against ignore patterns - if it *would* be ignored during linking, don't try to unlink it
			for _, pattern := range ignorePatterns {
				matched, matchErr := filepath.Match(pattern, relPath)
				if matchErr != nil {
					fmt.Printf("Warning: Invalid pattern '%s' in .gslm-ignore for package %s during unlink: %v\n", pattern, name, matchErr)
					continue
				}
				if matched {
					// This path would have been ignored during linking, so don't process for unlinking
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}

			targetPath := filepath.Join(l.TargetDir, relPath)

			// --- Unlinking Logic ---

			// 1. Check if target exists (os.Lstat)
			// 2. If exists:
			//    a. Check if it's a symlink.
			//    b. If yes, check if it points to the expected sourcePath.
			//    c. If yes, remove the symlink (os.Remove).
			//    d. Otherwise (not a symlink, or points elsewhere), ignore it.
			// 3. If not exists, ignore it.

			targetFi, err := os.Lstat(targetPath)
			if err != nil {
				if os.IsNotExist(err) {
					// Target doesn't exist, nothing to unlink for this path
					return nil
				}
				// Other error stat-ing target
				return fmt.Errorf("failed to stat target path %s: %w", targetPath, err)
			}

			// Target exists, check if it's a symlink pointing to our source
			if targetFi.Mode()&os.ModeSymlink != 0 {
				linkTarget, readErr := os.Readlink(targetPath)
				if readErr != nil {
					// Error reading link, maybe log this? For now, return error.
					return fmt.Errorf("failed to read symlink %s: %w", targetPath, readErr)
				}

				absSourcePath, absErr := filepath.Abs(sourcePath)
				if absErr != nil {
					return fmt.Errorf("failed to get absolute path for source %s: %w", sourcePath, absErr)
				}
				// Assume linkTarget is absolute as created by Link
				absLinkTarget, absErr := filepath.Abs(linkTarget)
				if absErr != nil {
					return fmt.Errorf("failed to get absolute path for link target %s: %w", linkTarget, absErr)
				}

				if absLinkTarget == absSourcePath {
					// This is the link we created, remove it
					fmt.Printf("Unlinking: %s (link to %s)\n", targetPath, sourcePath)
					if removeErr := os.Remove(targetPath); removeErr != nil {
						return fmt.Errorf("failed to remove symlink %s: %w", targetPath, removeErr)
					}
					// TODO: Optionally remove empty parent directories?
				} else {
					// fmt.Printf("  Paths do not match! Cannot unlink.\n") // Debug else branch
				}
			}
			// Else: Target exists but is not a symlink, ignore it.

			return nil // Continue walking
		})

		if err != nil {
			return fmt.Errorf("failed to unlink package %s: %w", name, err)
		}
	}

	return nil // Success
}
