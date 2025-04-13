package gslk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestDirs creates temporary source and target directories for testing.
// It returns the paths to the source dir, target dir, and a cleanup function.
func setupTestDirs(t *testing.T) (sourceDir string, targetDir string, cleanup func()) {
	tempDir, err := os.MkdirTemp("", "gslk_test_*")
	require.NoError(t, err, "Failed to create temp dir")

	sourceDir = filepath.Join(tempDir, "source")
	targetDir = filepath.Join(tempDir, "target")

	err = os.Mkdir(sourceDir, 0755)
	require.NoError(t, err, "Failed to create source dir")
	err = os.Mkdir(targetDir, 0755)
	require.NoError(t, err, "Failed to create target dir")

	cleanup = func() {
		os.RemoveAll(tempDir)
	}

	return sourceDir, targetDir, cleanup
}

// Helper to create dummy files/dirs for packages
func createDummyPackage(t *testing.T, pkgPath string, structure map[string]string) {
	for relPath, content := range structure {
		absPath := filepath.Join(pkgPath, relPath)
		parentDir := filepath.Dir(absPath)
		err := os.MkdirAll(parentDir, 0755)
		require.NoError(t, err, "Failed to create parent dir %s", parentDir)

		if content == "DIR" {
			err = os.Mkdir(absPath, 0755)
			require.NoError(t, err, "Failed to create dir %s", absPath)
		} else {
			err = os.WriteFile(absPath, []byte(content), 0644)
			require.NoError(t, err, "Failed to write file %s", absPath)
		}
	}
}

func TestLink(t *testing.T) {
	sourceDir, targetDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// Create a dummy package
	pkgName := "mypackage"
	pkgPath := filepath.Join(sourceDir, pkgName)
	err := os.Mkdir(pkgPath, 0755)
	require.NoError(t, err)

	dummyStructure := map[string]string{
		"file1.txt":        "content1",
		"subdir/file2.txt": "content2",
		"subdir/subsub":    "DIR",
	}
	createDummyPackage(t, pkgPath, dummyStructure)

	linker := &Linker{
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// Perform the link operation
	err = linker.Link([]string{pkgName})
	assert.NoError(t, err, "Link operation failed")

	// Verify the links
	for relPath := range dummyStructure {
		targetPath := filepath.Join(targetDir, relPath)
		sourcePath := filepath.Join(pkgPath, relPath)
		absSourcePath, _ := filepath.Abs(sourcePath) // Ignore error for simplicity in test

		// Check if target exists and is a symlink
		fi, err := os.Lstat(targetPath)
		assert.NoError(t, err, "Failed to stat target path %s", targetPath)

		// If the source was a directory, the target should be a directory
		sourceFi, _ := os.Lstat(sourcePath)
		if sourceFi.IsDir() {
			assert.True(t, fi.IsDir(), "Target %s should be a directory, but it's not", targetPath)
		} else {
			// If the source was a file, the target should be a symlink to it
			assert.True(t, fi.Mode()&os.ModeSymlink != 0, "Target %s is not a symlink", targetPath)
			linkTarget, err := os.Readlink(targetPath)
			assert.NoError(t, err, "Failed to read link %s", targetPath)
			assert.Equal(t, absSourcePath, linkTarget, "Link %s points to %s, expected %s", targetPath, linkTarget, absSourcePath)
		}
	}

	// Verify that the target directory structure was created
	_, err = os.Stat(filepath.Join(targetDir, "subdir", "subsub"))
	assert.NoError(t, err, "Subdirectory structure not created correctly in target")
}

func TestUnlink(t *testing.T) {
	sourceDir, targetDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// --- Setup: Create and link a package ---
	pkgName := "package_to_unlink"
	pkgPath := filepath.Join(sourceDir, pkgName)
	err := os.Mkdir(pkgPath, 0755)
	require.NoError(t, err)

	dummyStructure := map[string]string{
		"config.conf":   "some config",
		"bin/script.sh": "echo hello",
		"data":          "DIR",
	}
	createDummyPackage(t, pkgPath, dummyStructure)

	// Add an unrelated file in the target dir to ensure it's untouched
	unrelatedFilePath := filepath.Join(targetDir, "unrelated.txt")
	err = os.WriteFile(unrelatedFilePath, []byte("do not delete"), 0644)
	require.NoError(t, err)

	linker := &Linker{
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// Link it first
	err = linker.Link([]string{pkgName})
	require.NoError(t, err, "Pre-unlink Link operation failed")

	// Quick check link exists (optional)
	_, err = os.Lstat(filepath.Join(targetDir, "config.conf"))
	require.NoError(t, err, "Link target check failed before unlink")

	// --- Test: Perform the unlink operation ---
	err = linker.Unlink([]string{pkgName})
	assert.NoError(t, err, "Unlink operation failed")

	// --- Verification: Check links are removed ---
	for relPath := range dummyStructure {
		targetPath := filepath.Join(targetDir, relPath)
		sourcePath := filepath.Join(pkgPath, relPath)

		// Check if target exists - it should NOT exist if it was a file symlink
		sourceFi, _ := os.Lstat(sourcePath)
		if !sourceFi.IsDir() {
			_, err := os.Lstat(targetPath)
			assert.True(t, os.IsNotExist(err), "Target file link %s should not exist after unlink, but stat error is: %v", targetPath, err)
		} else {
			// For now, we don't assert that directories are removed by Unlink
			// _, err := os.Lstat(targetPath)
			// assert.True(t, os.IsNotExist(err), "Target directory %s should not exist after unlink, but stat error is: %v", targetPath, err)
		}
	}

	// Verify the unrelated file is still there
	_, err = os.Stat(unrelatedFilePath)
	assert.NoError(t, err, "Unrelated file %s was removed during unlink", unrelatedFilePath)

	// Optional: Check if empty directories created during linking were removed.
	// Our current Unlink doesn't do this, so `data` and `bin` might still exist.
	// _, err = os.Stat(filepath.Join(targetDir, "bin"))
	// assert.True(t, os.IsNotExist(err), "Empty directory 'bin' should ideally be removed")
	// _, err = os.Stat(filepath.Join(targetDir, "data"))
	// assert.True(t, os.IsNotExist(err), "Empty directory 'data' should ideally be removed")
}

func TestLinkConflict(t *testing.T) {
	sourceDir, targetDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// --- Setup: Create package and conflicting file ---
	pkgName := "pkg_with_conflict"
	pkgPath := filepath.Join(sourceDir, pkgName)
	err := os.Mkdir(pkgPath, 0755)
	require.NoError(t, err)

	dummyStructure := map[string]string{
		"file.txt": "source content",
		"dir":      "DIR",
	}
	createDummyPackage(t, pkgPath, dummyStructure)

	// Create conflicting items in the target directory *before* linking
	conflictingFilePath := filepath.Join(targetDir, "file.txt")
	conflictingFileContent := "target content - conflict"
	err = os.WriteFile(conflictingFilePath, []byte(conflictingFileContent), 0644)
	require.NoError(t, err, "Failed to create conflicting file")

	conflictingDirPath := filepath.Join(targetDir, "dir")
	err = os.Mkdir(conflictingDirPath, 0755)
	require.NoError(t, err, "Failed to create conflicting dir")
	// Add a file inside the conflicting dir to ensure it's not empty/overwritten
	err = os.WriteFile(filepath.Join(conflictingDirPath, "inner.txt"), []byte("inner"), 0644)
	require.NoError(t, err)

	linker := &Linker{
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// --- Test: Attempt to link, expecting conflict errors ---
	err = linker.Link([]string{pkgName})

	// --- Verification: Check for error and that conflicts remain ---
	assert.Error(t, err, "Link should have returned an error due to conflict")
	if err != nil {
		// Check if the error message contains mentions of the conflict
		assert.Contains(t, err.Error(), "conflict: target", "Error message should indicate a conflict")
		// We expect the error on the first conflict found (`dir` or `file.txt`)
		assert.Contains(t, err.Error(), filepath.Join(targetDir, ""), "Error message should contain conflicting target path")
	}

	// Verify the conflicting file in target is unchanged
	content, readErr := os.ReadFile(conflictingFilePath)
	assert.NoError(t, readErr, "Failed to read conflicting file after link attempt")
	assert.Equal(t, conflictingFileContent, string(content), "Conflicting file content was modified")

	// Verify the conflicting dir in target is unchanged
	_, statErr := os.Stat(filepath.Join(conflictingDirPath, "inner.txt"))
	assert.NoError(t, statErr, "File inside conflicting dir is missing after link attempt")
}

// Test case where the target already exists but is the correct symlink
func TestLinkAlreadyLinked(t *testing.T) {
	sourceDir, targetDir, cleanup := setupTestDirs(t)
	defer cleanup()

	// --- Setup: Create and link a package ---
	pkgName := "prelinked_pkg"
	pkgPath := filepath.Join(sourceDir, pkgName)
	err := os.Mkdir(pkgPath, 0755)
	require.NoError(t, err)

	dummyStructure := map[string]string{
		"link_me.txt": "link content",
	}
	createDummyPackage(t, pkgPath, dummyStructure)

	linker := &Linker{
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// Link it once
	err = linker.Link([]string{pkgName})
	require.NoError(t, err, "First Link operation failed")

	// --- Test: Link it again ---
	err = linker.Link([]string{pkgName})

	// --- Verification: No error should occur ---
	assert.NoError(t, err, "Linking an already correctly linked package should not produce an error")

	// Verify the link is still correct (optional but good practice)
	targetPath := filepath.Join(targetDir, "link_me.txt")
	sourcePath := filepath.Join(pkgPath, "link_me.txt")
	absSourcePath, _ := filepath.Abs(sourcePath)

	fi, err := os.Lstat(targetPath)
	require.NoError(t, err)
	require.True(t, fi.Mode()&os.ModeSymlink != 0)
	linkTarget, err := os.Readlink(targetPath)
	require.NoError(t, err)
	assert.Equal(t, absSourcePath, linkTarget, "Link %s points to %s, expected %s", targetPath, linkTarget, absSourcePath)
}

func TestLinkWithIgnore(t *testing.T) {
	sourceDir, targetDir, cleanup := setupTestDirs(t)
	defer cleanup()

	pkgName := "ignore_pkg"
	pkgPath := filepath.Join(sourceDir, pkgName)
	err := os.Mkdir(pkgPath, 0755)
	require.NoError(t, err)

	// Create .gslk-ignore file
	ignoreContent := "ignored_file.txt\nignored_dir\n*.tmp\npattern/specific_file.dat\n"
	ignoreFilePath := filepath.Join(pkgPath, ".gslk-ignore")
	err = os.WriteFile(ignoreFilePath, []byte(ignoreContent), 0644)
	require.NoError(t, err, "Failed to create .gslk-ignore file")

	// Create package structure including ignored and non-ignored items
	dummyStructure := map[string]string{
		"file_to_link.txt":          "link me",
		"ignored_file.txt":          "ignore me",
		"ignored_dir/file.txt":      "ignore me too",
		"ignored_dir/sub":           "DIR",
		"another_dir/file.txt":      "link this one",
		"some_file.tmp":             "temporary, ignore",
		"pattern/another_file.txt":  "link me pattern",
		"pattern/specific_file.dat": "ignore me pattern",
	}
	createDummyPackage(t, pkgPath, dummyStructure)

	linker := &Linker{
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// --- Test: Perform Link ---
	err = linker.Link([]string{pkgName})
	assert.NoError(t, err, "Link operation with ignores failed")

	// --- Verification ---
	// Files/Dirs that SHOULD be linked
	shouldLink := []string{
		"file_to_link.txt",
		"another_dir/file.txt",
		"pattern/another_file.txt",
	}
	for _, relPath := range shouldLink {
		targetPath := filepath.Join(targetDir, relPath)
		sourcePath := filepath.Join(pkgPath, relPath)
		absSourcePath, _ := filepath.Abs(sourcePath)

		fi, err := os.Lstat(targetPath)
		assert.NoError(t, err, "Should link: Failed to stat target path %s", targetPath)
		require.NotNil(t, fi, "Should link: Stat result is nil for %s", targetPath)
		assert.True(t, fi.Mode()&os.ModeSymlink != 0, "Should link: Target %s is not a symlink", targetPath)
		linkTarget, err := os.Readlink(targetPath)
		assert.NoError(t, err, "Should link: Failed to read link %s", targetPath)
		assert.Equal(t, absSourcePath, linkTarget, "Should link: Link %s points to %s, expected %s", targetPath, linkTarget, absSourcePath)
	}

	// Files/Dirs that SHOULD be ignored
	shouldIgnore := []string{
		".gslk-ignore", // The ignore file itself
		"ignored_file.txt",
		"ignored_dir/file.txt",
		"ignored_dir/sub",
		"ignored_dir", // The directory itself
		"some_file.tmp",
		"pattern/specific_file.dat",
	}
	for _, relPath := range shouldIgnore {
		targetPath := filepath.Join(targetDir, relPath)
		_, err := os.Lstat(targetPath)
		assert.True(t, os.IsNotExist(err), "Should ignore: Target %s should not exist but it does (stat err: %v)", targetPath, err)
	}
}

func TestUnlinkWithIgnore(t *testing.T) {
	sourceDir, targetDir, cleanup := setupTestDirs(t)
	defer cleanup()

	pkgName := "unlink_ignore_pkg"
	pkgPath := filepath.Join(sourceDir, pkgName)
	err := os.Mkdir(pkgPath, 0755)
	require.NoError(t, err)

	// Create .gslk-ignore file
	ignoreContent := "config/secrets.yml\nlogs\n"
	ignoreFilePath := filepath.Join(pkgPath, ".gslk-ignore")
	err = os.WriteFile(ignoreFilePath, []byte(ignoreContent), 0644)
	require.NoError(t, err, "Failed to create .gslk-ignore file")

	// Create package structure
	dummyStructure := map[string]string{
		"config/app.yml":     "app config",
		"config/secrets.yml": "secret stuff", // Ignored
		"bin/run.sh":         "run script",
		"logs/debug.log":     "debug info", // Ignored
		"logs/sub/trace.log": "trace info", // Ignored (part of ignored dir)
	}
	createDummyPackage(t, pkgPath, dummyStructure)

	linker := &Linker{
		SourceDir: sourceDir,
		TargetDir: targetDir,
	}

	// --- Setup: Link the package first (respecting ignores) ---
	err = linker.Link([]string{pkgName})
	require.NoError(t, err, "Pre-unlink Link operation failed")

	// Quick check: ensure linked file exists, ignored file doesn't
	_, err = os.Lstat(filepath.Join(targetDir, "config/app.yml"))
	require.NoError(t, err, "Linked file does not exist after link")
	_, err = os.Lstat(filepath.Join(targetDir, "config/secrets.yml"))
	require.True(t, os.IsNotExist(err), "Ignored file exists after link")
	_, err = os.Lstat(filepath.Join(targetDir, "logs"))
	require.True(t, os.IsNotExist(err), "Ignored directory exists after link")

	// --- Test: Perform Unlink ---
	err = linker.Unlink([]string{pkgName})
	assert.NoError(t, err, "Unlink operation with ignores failed")

	// --- Verification ---
	// Files/Dirs that SHOULD have been linked AND thus unlinked
	shouldBeUnlinked := []string{
		"config/app.yml",
		"bin/run.sh",
		// The parent dirs might remain, depending on Unlink logic for empty dirs
	}
	for _, relPath := range shouldBeUnlinked {
		targetPath := filepath.Join(targetDir, relPath)
		_, err := os.Lstat(targetPath)
		assert.True(t, os.IsNotExist(err), "Should be unlinked: Target %s should not exist but it does (stat err: %v)", targetPath, err)
	}

	// Files/Dirs that SHOULD have been ignored (and thus never existed in target)
	shouldBeIgnored := []string{
		".gslk-ignore",
		"config/secrets.yml",
		"logs/debug.log",
		"logs/sub/trace.log",
		"logs",
	}
	for _, relPath := range shouldBeIgnored {
		targetPath := filepath.Join(targetDir, relPath)
		_, err := os.Lstat(targetPath)
		assert.True(t, os.IsNotExist(err), "Should be ignored: Target %s should not exist (stat err: %v)", targetPath, err)
	}
}
