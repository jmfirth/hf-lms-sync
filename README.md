# Hugging Face to LM Studio Sync

An interactive CLI tool for synchronizing your Hugging Face model cache with the LM Studio models cache. Built with Go, Bubble Tea, and Bubbles, this tool provides a modern terminal UI that makes it easy to link, unlink, and purge model caches across platforms.

## Introduction

Hugging Face to LM Studio Sync helps users keep their model caches in sync. It scans the Hugging Face cache for available models, detects stale links in the LM Studio cache, and provides an interactive terminal UI to manage linking and purging operations. It’s cross-platform, supporting macOS (Intel & Apple Silicon), Linux, and Windows.

## Features

- **Interactive Terminal UI:**  
  A dynamic, scrollable list of models with a persistent title bar and command bar displaying available hotkeys.

- **Cross-Platform Support:**  
  Automatically detects cache directories based on the operating system, ensuring seamless operation on macOS, Windows, and Linux.

- **Command Operations:**  
  Link individual models, unlink models, purge stale links, and perform bulk operations (link all, unlink all, purge all) directly from the CLI.

## Getting Started

### Prerequisites

- [Go](https://golang.org/dl/) (version 1.20 or later recommended)
- A terminal that supports ANSI colors (for full UI experience)

### Installation

1. **Clone the repository:**

   ```bash
   git clone https://github.com/yourusername/hf-lms-sync.git
   cd hf-lms-sync
   ```

2. **Build the executable:**

   ```bash
   go build -o hf-lms-sync ./cmd/hf-lms-sync
   ```

3. **(Optional) Install prebuilt binaries:**  
   Binaries for different platforms are available via GitHub Releases.

### Usage

Run the tool from your terminal:

```bash
./hf-lms-sync [options] [target_directory]
```

#### Options

- `--verbose`: Enable detailed logging to `hf-lmfs-sync.log` in the current directory. Log messages are written to the file only, not to the console, to avoid disrupting the terminal UI.
- `--help`: Display usage information

#### Basic Operation

- If no `target_directory` is provided, the tool will automatically determine the LM Studio models cache directory based on your operating system.
- Use the arrow keys (or `j`/`k`) to navigate through the list.
- Available commands (displayed in the command bar):
  - **l**: Link the selected model.
  - **u**: Unlink the selected model.
  - **c**: Purge (clean) the selected model if stale.
  - **L**: Link all unlinked models.
  - **U**: Unlink all linked models.
  - **C**: Purge all stale links.
  - **q**: Quit the application.

## Development

### Setting Up the Project

1. **Clone the repository:**

   ```bash
   git clone https://github.com/yourusername/hf-lms-sync.git
   cd hf-lms-sync
   ```

2. **Initialize Go Modules (if not already initialized):**

   ```bash
   go mod tidy
   ```

3. **VS Code Configuration:**

   - **Tasks:**  
     A sample `.vscode/tasks.json` is provided for building and running the project.
   - **Launch Configuration:**  
     Use `.vscode/launch.json` to debug the project with the Go extension.
   - **Testing:**  
     Tests can be run using the built-in test explorer or via the command line:
     ```bash
     go test ./...
     ```

### Testing

A comprehensive suite of tests is located in the `internal/fsutils` and `internal/ui` directories. Run all tests with:

```bash
go test ./...
```

## Contributing

Contributions are welcome! Please open issues or submit pull requests on GitHub. Be sure to follow the coding style and add tests for any new features or bug fixes.

## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE.txt) file for details.
