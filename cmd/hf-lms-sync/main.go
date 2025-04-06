// cmd/hf-lms-sync/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmfirth/hf-lms-sync/internal/fsutils"
	"github.com/jmfirth/hf-lms-sync/internal/logger"
	"github.com/jmfirth/hf-lms-sync/internal/ui"
)

// printUsage prints the program usage information and exits
func printUsage() {
	fmt.Println("Hugging Face to LM Studio Sync")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  hf-lms-sync [options] [target_directory]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  --verbose    Enable detailed logging to hf-lmfs-sync.log in the current directory")
	fmt.Println("  --help       Display this help message")
	fmt.Println("")
	fmt.Println("If no target_directory is provided, the tool will automatically determine")
	fmt.Println("the LM Studio models cache directory based on your operating system.")
	os.Exit(0)
}

func main() {
	// Define command line flags
	verboseFlag := flag.Bool("verbose", false, "Enable verbose logging to file")
	helpFlag := flag.Bool("help", false, "Display help message")
	
	// Parse flags
	flag.Parse()
	
	// Show help if requested
	if *helpFlag {
		printUsage()
	}

	// Initialize the logger
	appLogger, err := logger.New(*verboseFlag)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer appLogger.Close()

	// Determine LM Studio Models directory.
	args := flag.Args()
	var targetDir string
	if len(args) > 0 {
		targetDir = args[0]
		if *verboseFlag {
			appLogger.Info("MAIN", "Using provided target directory: %s", targetDir)
		}
	} else {
		var err error
		targetDir, err = fsutils.GetLmStudioModelsDir()
		if err != nil {
			appLogger.Error("MAIN", "Error determining LM Studio Models directory: %v", err)
			log.Fatalf("Error determining LM Studio Models directory: %v", err)
		}
		if *verboseFlag {
			appLogger.Info("MAIN", "Using default LM Studio Models directory: %s", targetDir)
		}
	}

	// Determine and print Hugging Face cache directory.
	hfCacheDir, err := fsutils.GetHfCacheDir()
	if err != nil {
		appLogger.Error("MAIN", "Error determining Hugging Face cache directory: %v", err)
		log.Fatalf("Error determining Hugging Face cache directory: %v", err)
	}
	
	if *verboseFlag {
		appLogger.Info("MAIN", "Hugging Face cache directory: %s", hfCacheDir)
		appLogger.Info("MAIN", "Starting UI with target directory: %s", targetDir)
	}

	// Start the Bubble Tea program with the logger
	p := tea.NewProgram(ui.New(targetDir, appLogger))
	if err := p.Start(); err != nil {
		appLogger.Error("MAIN", "Error running program: %v", err)
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
	
	if *verboseFlag {
		appLogger.Info("MAIN", "Application terminated normally")
	}
}
