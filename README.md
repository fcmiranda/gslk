gslk - Go Symlink

`gslk` is a simple command-line utility written in Go to manage symbolic links for configuration packages, often used for managing dotfiles. It allows you to link files and directories from a source location (containing your managed packages) to a target location (e.g., your home directory).

## Features

*   Links files from source package directories to a target directory.
*   Unlinks previously created symlinks.
*   Detects and prevents overwriting existing files/directories in the target location (unless they are the correct symlink).
*   Supports ignoring specific files or patterns within packages via a `.gslk-ignore` file.

## Usage

```bash
gslk [options] <package1> [package2...]
```

**Actions (Options):**

*   Default action is to link packages.
*   `-D`: Unlink/delete packages instead of linking.
*   `-GL` or `--gslk`: Explicitly specify linking packages (default action).
*   `-R`: Relink packages (unlink then link).

**Required Options:**

*   `-s` or `--source`: The source directory containing your configuration packages (subdirectories).

**Additional Options:**

*   `-t` or `--target`: The target directory where the symlinks should be created or removed (default: `$HOME`).
*   `-n`: Dry run: show what would be done without actually doing it.
*   `-v`: Increase verbosity.

**Arguments:**

*   `<package1> [package2...]`: One or more names of the package subdirectories within `<source_dir>` to process.

**Examples:**

To link the `zsh`, `vim`, and `git` configuration packages from a `./dotfiles` directory to your home directory (`$HOME`):

```bash
gslk -s ./dotfiles zsh vim git
```

The above command uses the default target directory (`$HOME`). To specify a different target directory:

```bash
gslk -s ./dotfiles -t /path/to/target zsh vim git
```

To explicitly link packages:
```bash
gslk -GL -s ./dotfiles zsh vim git
```

Or using the long-form flag:
```bash
gslk --gslk -s ./dotfiles zsh vim git
```

To unlink the `vim` package:

```bash
gslk -D -s ./dotfiles vim
```

To relink (unlink then link) the `vim` package verbosely:

```bash
gslk -R -v -s ./dotfiles vim
```

To perform a dry run showing what would happen without making changes:

```bash
gslk -n -s ./dotfiles zsh vim git
```

## Packages

`gslk` treats each subdirectory within the specified `<source_dir>` as a "package". When you run `gslk link`, it walks through the files and directories within each specified package directory in the source.

## Ignoring Files (`.gslk-ignore`)

You can prevent certain files or directories within a package from being linked by creating a `.gslk-ignore` file in the root of that package directory (e.g., `<source_dir>/<package_name>/.gslk-ignore`).

This file works like a `.gitignore` file. Each line specifies a pattern:

*   Lines starting with `#` are comments.
*   Blank lines are ignored.
*   Other lines are treated as file patterns (using `filepath.Match` syntax) relative to the package directory.

**Example `.gslk-ignore`:**

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

To build the `gslk` executable:

```bash
go build ./cmd/gslk
```

Alternatively, you can specify the output file and the main Go file directly:

```bash
go build -o gslk cmd/gslk/main.go
```

This will create the `gslk` binary in the current directory.