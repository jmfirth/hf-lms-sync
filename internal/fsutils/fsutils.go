// internal/fsutils/fsutils.go
package fsutils

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	metadataFile = ".hf-lms-sync"
	snapshotsDir = "snapshots"
)

// GetHfCacheDir returns the path to the Hugging Face cache directory based on the OS.
func GetHfCacheDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "huggingface", "hub"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Local", "huggingface", "hub"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", "huggingface", "hub"), nil
	default:
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache != "" {
			return filepath.Join(xdgCache, "huggingface", "hub"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", "huggingface", "hub"), nil
	}
}

// GetLmStudioModelsDir returns the path to the LM Studio models directory based on the OS.
func GetLmStudioModelsDir() (string, error) {
	switch runtime.GOOS {
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			return filepath.Join(localAppData, "lm-studio", "models"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Local", "lm-studio", "models"), nil
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", "lm-studio", "models"), nil
	default:
		xdgCache := os.Getenv("XDG_CACHE_HOME")
		if xdgCache != "" {
			return filepath.Join(xdgCache, "lm-studio", "models"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".cache", "lm-studio", "models"), nil
	}
}

// ModelInfo represents a model and its file paths.
type ModelInfo struct {
	CacheDirName     string
	OrganizationName string
	ModelName        string
	SourcePath       string
	TargetPath       string
	IsLinked         bool
	IsStale          bool
	StaleReason      string
}

// verifySymlinks checks if all symlinks in a directory are valid
func verifySymlinks(dir string) bool {
	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		return false
	}
	
	for _, entry := range entries {
		if entry.Mode()&os.ModeSymlink != 0 {
			path := filepath.Join(dir, entry.Name())
			if _, err := os.Readlink(path); err != nil {
				return false
			}
		}
	}
	return true
}

// LoadModels scans the Hugging Face cache directory for model directories and returns a slice of ModelInfo.
func LoadModels(targetDir string) ([]ModelInfo, error) {
	hfCache, err := GetHfCacheDir()
	if err != nil {
		return nil, err
	}

	if info, err := os.Stat(hfCache); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("HuggingFace cache directory does not exist or is not a directory: %s", hfCache)
	}
	if info, err := os.Stat(targetDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("Target directory does not exist or is not a directory: %s", targetDir)
	}

	entries, err := ioutil.ReadDir(hfCache)
	if err != nil {
		return nil, err
	}

	var models []ModelInfo
	for _, entry := range entries {
		if !entry.IsDir() || !strings.Contains(entry.Name(), "--") {
			continue
		}
		parts := strings.Split(entry.Name(), "--")
		if len(parts) < 2 {
			continue
		}
		organization := parts[len(parts)-2]
		modelName := parts[len(parts)-1]
		sourcePath := filepath.Join(hfCache, entry.Name())
		targetPath := filepath.Join(targetDir, organization, modelName)
		isLinked := false
		if _, err := os.Stat(filepath.Join(targetPath, metadataFile)); err == nil {
			// Only mark as linked if both metadata file exists and symlinks are valid
			isLinked = verifySymlinks(targetPath)
		}
		models = append(models, ModelInfo{
			CacheDirName:     entry.Name(),
			OrganizationName: organization,
			ModelName:        modelName,
			SourcePath:       sourcePath,
			TargetPath:       targetPath,
			IsLinked:         isLinked,
		})
	}

	return models, nil
}

// FindStaleLinks recursively walks the target directory and identifies linked directories whose source no longer exists.
func FindStaleLinks(targetDir string) ([]ModelInfo, error) {
	var stale []ModelInfo
	err := filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Look for directories that contain the metadata file.
		if d.IsDir() {
			metadataPath := filepath.Join(path, metadataFile)
			if _, err := os.Stat(metadataPath); err == nil {
				parentDir := filepath.Dir(path)
				organization := filepath.Base(parentDir)
				modelName := filepath.Base(path)
				cacheDirName := "models--" + organization + "--" + modelName
				hfCache, err := GetHfCacheDir()
				if err != nil {
					return err
				}
				sourcePath := filepath.Join(hfCache, cacheDirName, snapshotsDir)
				if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
					stale = append(stale, ModelInfo{
						CacheDirName:     cacheDirName,
						OrganizationName: organization,
						ModelName:        modelName,
						SourcePath:       sourcePath,
						TargetPath:       path,
						IsLinked:         true,
						IsStale:          true,
						StaleReason:      "Source directory not found",
					})
				}
			}
		}
		return nil
	})
	return stale, err
}

// LinkModel creates symlinks from the snapshot files in the source to the target directory and writes a metadata file.
func LinkModel(m ModelInfo) error {
	if info, err := os.Stat(m.SourcePath); err != nil || !info.IsDir() {
		return fmt.Errorf("source path %s does not exist or is not a directory", m.SourcePath)
	}
	snapshotsPath := filepath.Join(m.SourcePath, snapshotsDir)
	if info, err := os.Stat(snapshotsPath); err != nil || !info.IsDir() {
		return fmt.Errorf("snapshots directory %s does not exist", snapshotsPath)
	}
	snapshotDirs, err := ioutil.ReadDir(snapshotsPath)
	if err != nil {
		return err
	}
	
	// Clean up existing target directory if it exists
	if _, err := os.Stat(m.TargetPath); err == nil {
		if err := os.RemoveAll(m.TargetPath); err != nil {
			return fmt.Errorf("failed to clean up existing target directory: %v", err)
		}
	}
	
	if err := os.MkdirAll(m.TargetPath, 0755); err != nil {
		return err
	}
	
	for _, snapDir := range snapshotDirs {
		if !snapDir.IsDir() {
			continue
		}
		snapPath := filepath.Join(snapshotsPath, snapDir.Name())
		files, err := ioutil.ReadDir(snapPath)
		if err != nil {
			return err
		}
		for _, file := range files {
			src := filepath.Join(snapPath, file.Name())
			dst := filepath.Join(m.TargetPath, file.Name())
			
			// Always try to resolve the real source file
			realSource, err := filepath.EvalSymlinks(src)
			if err != nil {
				return fmt.Errorf("failed to resolve symlink for %s: %v", src, err)
			}
			
			if err := os.Symlink(realSource, dst); err != nil {
				return fmt.Errorf("failed to create symlink from %s to %s: %v", realSource, dst, err)
			}
		}
	}
	metadataContent := []byte(time.Now().Format(time.RFC3339))
	return ioutil.WriteFile(filepath.Join(m.TargetPath, metadataFile), metadataContent, 0644)
}

// UnlinkModel removes the target directory if it contains the metadata file.
func UnlinkModel(m ModelInfo) error {
	metadataPath := filepath.Join(m.TargetPath, metadataFile)
	if _, err := os.Stat(metadataPath); err == nil {
		return os.RemoveAll(m.TargetPath)
	}
	return nil
}
