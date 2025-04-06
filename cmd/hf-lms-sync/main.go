// cmd/hf-lms-sync/main.go
package main

import (
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jmfirth/hf-lms-sync/internal/fsutils"
	"github.com/jmfirth/hf-lms-sync/internal/ui"
)

func main() {
	// Determine LM Studio Models directory.
	var targetDir string
	if len(os.Args) > 1 {
		targetDir = os.Args[1]
	} else {
		var err error
		targetDir, err = fsutils.GetLmStudioModelsDir()
		if err != nil {
			log.Fatalf("Error determining LM Studio Models directory: %v", err)
		}
	}

	// Determine and print Hugging Face cache directory.
	_, err := fsutils.GetHfCacheDir()
	if err != nil {
		log.Fatalf("Error determining Hugging Face cache directory: %v", err)
	}

	// Start the Bubble Tea program.
	p := tea.NewProgram(ui.New(targetDir))
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
