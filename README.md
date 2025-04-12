# gslm# gslm - Go Symlink Manager

`gslm` is a simple command-line utility written in Go to manage symbolic links for configuration packages, often used for managing dotfiles. It allows you to link files and directories from a source location (containing your managed packages) to a target location (e.g., your home directory).

## Features

*   Links files from source package directories to a target directory.
*   Unlinks previously created symlinks.
*   Detects and prevents overwriting existing files/directories in the target location (unless they are the correct symlink).
*   Supports ignoring specific files or patterns within packages via a `.gslm-ignore` file.

## Usage

```bash
gslm <action> -s <source_dir> -t <target_dir> <package1> [package2...]
```

**Actions:**

*   `link`: Creates symlinks in the `<target_dir>` for the specified packages found in `<source_dir>`.
*   `unlink`: Removes symlinks from the `<target_dir>` that correspond to the specified packages found in `<source_dir>`.

**Options:**

*   `-s`: (Required) The source directory containing your configuration packages (subdirectories).
*   `-t`: (Required) The target directory where the symlinks should be created or removed.

**Arguments:**

*   `<package1> [package2...]`: One or more names of the package subdirectories within `<source_dir>` to process.

**Example:**

To link the `zsh`, `vim`, and `git` configuration packages from a `./dotfiles` directory to your home directory (`$HOME`):

```bash
gslm link -s ./dotfiles -t $HOME zsh vim git
```

To unlink the `vim` package:

```bash
gslm unlink -s ./dotfiles -t $HOME vim
```

## Packages

`gslm` treats each subdirectory within the specified `<source_dir>` as a "package". When you run `gslm link`, it walks through the files and directories within each specified package directory in the source.

## Ignoring Files (`.gslm-ignore`)

You can prevent certain files or directories within a package from being linked by creating a `.gslm-ignore` file in the root of that package directory (e.g., `<source_dir>/<package_name>/.gslm-ignore`).

This file works like a `.gitignore` file. Each line specifies a pattern:

*   Lines starting with `#` are comments.
*   Blank lines are ignored.
*   Other lines are treated as file patterns (using `filepath.Match` syntax) relative to the package directory.

**Example `.gslm-ignore`:**

```
# Ignore secret files
secrets.yml
*.key

# Ignore build artifacts
build/
*.o

# Ignore log directories
logs/
```

Files and directories matching these patterns will be skipped during both `link` and `unlink` operations.

## Building

To build the `gslm` executable:

```bash
go build ./cmd/gslm
```

Alternatively, you can specify the output file and the main Go file directly:

```bash
go build -o gslm cmd/gslm/main.go
```

This will create the `gslm` binary in the current directory.