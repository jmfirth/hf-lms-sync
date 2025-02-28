package fsutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestGetHfCacheDir_XDG tests that when XDG_CACHE_HOME is set (on Linux),
// GetHfCacheDir returns the expected path.
func TestGetHfCacheDir_XDG(t *testing.T) {
	// Only applicable for Linux.
	if runtime.GOOS != "linux" {
		t.Skip("Skipping XDG_CACHE_HOME test on non-Linux OS")
	}

	tempDir, err := ioutil.TempDir("", "xdg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("XDG_CACHE_HOME", tempDir)
	defer os.Unsetenv("XDG_CACHE_HOME")

	expected := filepath.Join(tempDir, "huggingface", "hub")
	dir, err := GetHfCacheDir()
	if err != nil {
		t.Fatalf("GetHfCacheDir returned error: %v", err)
	}
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

// TestGetLmStudioModelsDir_XDG tests that when XDG_CACHE_HOME is set (on Linux),
// GetLmStudioModelsDir returns the expected path.
func TestGetLmStudioModelsDir_XDG(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping XDG_CACHE_HOME test on non-Linux OS")
	}

	tempDir, err := ioutil.TempDir("", "xdg")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	os.Setenv("XDG_CACHE_HOME", tempDir)
	defer os.Unsetenv("XDG_CACHE_HOME")

	expected := filepath.Join(tempDir, "lm-studio", "models")
	dir, err := GetLmStudioModelsDir()
	if err != nil {
		t.Fatalf("GetLmStudioModelsDir returned error: %v", err)
	}
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

// TestLoadModels creates an isolated environment by overriding HOME and XDG_CACHE_HOME,
// then simulates a Hugging Face cache with a single dummy model directory.
func TestLoadModels(t *testing.T) {
	// Create a temporary directory to simulate the user's home.
	tempHome, err := ioutil.TempDir("", "home")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempHome)

	// Override HOME and XDG_CACHE_HOME so that fsutils functions use our temp directory.
	os.Setenv("HOME", tempHome)
	os.Setenv("XDG_CACHE_HOME", tempHome)
	defer os.Unsetenv("HOME")
	defer os.Unsetenv("XDG_CACHE_HOME")

	// Determine expected Hugging Face cache directory.
	var hfHubDir string
	if runtime.GOOS == "darwin" {
		// On macOS, GetHfCacheDir returns HOME/.cache/huggingface/hub.
		hfHubDir = filepath.Join(tempHome, ".cache", "huggingface", "hub")
	} else {
		// On Linux (with XDG_CACHE_HOME set), GetHfCacheDir returns XDG_CACHE_HOME/huggingface/hub.
		hfHubDir = filepath.Join(tempHome, "huggingface", "hub")
	}

	if err := os.MkdirAll(hfHubDir, 0755); err != nil {
		t.Fatalf("failed to create hf hub directory: %v", err)
	}

	// Create a dummy model directory with name "models--org--model".
	modelDirName := "models--org--model"
	modelDirPath := filepath.Join(hfHubDir, modelDirName)
	if err := os.Mkdir(modelDirPath, 0755); err != nil {
		t.Fatalf("failed to create model directory: %v", err)
	}

	// Create a temporary target directory.
	targetDir, err := ioutil.TempDir("", "target")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(targetDir)

	models, err := LoadModels(targetDir)
	if err != nil {
		t.Fatalf("LoadModels returned error: %v", err)
	}
	if len(models) != 1 {
		t.Errorf("expected 1 model, got %d", len(models))
	}
	model := models[0]
	if model.OrganizationName != "org" {
		t.Errorf("expected organization 'org', got %s", model.OrganizationName)
	}
	if model.ModelName != "model" {
		t.Errorf("expected model name 'model', got %s", model.ModelName)
	}
	expectedTarget := filepath.Join(targetDir, "org", "model")
	if model.TargetPath != expectedTarget {
		t.Errorf("expected target path %q, got %q", expectedTarget, model.TargetPath)
	}
}

// TestFindStaleLinks simulates a target directory with a stale link.
// It creates a target structure with a metadata file but without a corresponding source.
func TestFindStaleLinks(t *testing.T) {
	// Ensure HOME is set so that os.UserHomeDir() works.
	if os.Getenv("HOME") == "" {
		tempHome, err := ioutil.TempDir("", "home")
		if err != nil {
			t.Fatal(err)
		}
		os.Setenv("HOME", tempHome)
		defer os.Unsetenv("HOME")
		defer os.RemoveAll(tempHome)
	}

	targetDir, err := ioutil.TempDir("", "target")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(targetDir)

	// Simulate a linked model directory.
	orgDir := filepath.Join(targetDir, "org")
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		t.Fatalf("failed to create org directory: %v", err)
	}
	modelDir := filepath.Join(orgDir, "model")
	if err := os.Mkdir(modelDir, 0755); err != nil {
		t.Fatalf("failed to create model directory: %v", err)
	}
	// Write the metadata file.
	metadataPath := filepath.Join(modelDir, ".hf-lms-sync")
	if err := ioutil.WriteFile(metadataPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	stale, err := FindStaleLinks(targetDir)
	if err != nil {
		t.Fatalf("FindStaleLinks returned error: %v", err)
	}
	if len(stale) != 1 {
		t.Errorf("expected 1 stale link, got %d", len(stale))
	}
	found := stale[0]
	if !strings.Contains(found.CacheDirName, "org") || !strings.Contains(found.CacheDirName, "model") {
		t.Errorf("unexpected CacheDirName: %s", found.CacheDirName)
	}
}

// TestLinkAndUnlinkModel simulates linking and unlinking a model by creating a dummy
// snapshot structure in the source directory.
func TestLinkAndUnlinkModel(t *testing.T) {
	// Create temporary source and target directories.
	sourceDir, err := ioutil.TempDir("", "source")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sourceDir)
	targetDir, err := ioutil.TempDir("", "target")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(targetDir)

	// Create a snapshots directory in the source.
	snapshotsPath := filepath.Join(sourceDir, "snapshots")
	if err := os.Mkdir(snapshotsPath, 0755); err != nil {
		t.Fatalf("failed to create snapshots directory: %v", err)
	}
	// Create a dummy snapshot.
	snapshotDir := filepath.Join(snapshotsPath, "v1")
	if err := os.Mkdir(snapshotDir, 0755); err != nil {
		t.Fatalf("failed to create snapshot directory: %v", err)
	}
	// Create a dummy file in the snapshot.
	dummyFile := filepath.Join(snapshotDir, "dummy.txt")
	if err := ioutil.WriteFile(dummyFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("failed to create dummy file: %v", err)
	}

	// Create a ModelInfo instance.
	mInfo := ModelInfo{
		CacheDirName:     "models--org--model",
		OrganizationName: "org",
		ModelName:        "model",
		SourcePath:       sourceDir,
		TargetPath:       filepath.Join(targetDir, "org", "model"),
		IsLinked:         false,
	}

	// Test LinkModel.
	if err := LinkModel(mInfo); err != nil {
		t.Fatalf("LinkModel returned error: %v", err)
	}
	// Check that the target directory exists.
	if _, err := os.Stat(mInfo.TargetPath); err != nil {
		t.Errorf("target directory not created: %v", err)
	}
	// Check that the dummy file was symlinked.
	targetDummy := filepath.Join(mInfo.TargetPath, "dummy.txt")
	info, err := os.Lstat(targetDummy)
	if err != nil {
		t.Errorf("dummy file not found in target: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("expected dummy.txt to be a symlink")
	}

	// Test UnlinkModel.
	if err := UnlinkModel(mInfo); err != nil {
		t.Fatalf("UnlinkModel returned error: %v", err)
	}
	// Verify that the target directory has been removed.
	if _, err := os.Stat(mInfo.TargetPath); !os.IsNotExist(err) {
		t.Errorf("target directory still exists after unlink")
	}
}

// TestLinkModelErrorNoSource tests that LinkModel returns an error when the source directory does not exist.
func TestLinkModelErrorNoSource(t *testing.T) {
	targetDir, err := ioutil.TempDir("", "target")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(targetDir)

	// Provide a non-existent source directory.
	mInfo := ModelInfo{
		CacheDirName:     "models--org--model",
		OrganizationName: "org",
		ModelName:        "model",
		SourcePath:       filepath.Join(os.TempDir(), "nonexistent"),
		TargetPath:       filepath.Join(targetDir, "org", "model"),
		IsLinked:         false,
	}

	err = LinkModel(mInfo)
	if err == nil {
		t.Errorf("expected error from LinkModel when source does not exist, got nil")
	}
}

// TestLinkModelErrorNoSnapshots tests that LinkModel returns an error when the snapshots directory is missing.
func TestLinkModelErrorNoSnapshots(t *testing.T) {
	// Create a temporary source directory without a snapshots subdirectory.
	sourceDir, err := ioutil.TempDir("", "source")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(sourceDir)

	targetDir, err := ioutil.TempDir("", "target")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(targetDir)

	mInfo := ModelInfo{
		CacheDirName:     "models--org--model",
		OrganizationName: "org",
		ModelName:        "model",
		SourcePath:       sourceDir, // No snapshots folder here.
		TargetPath:       filepath.Join(targetDir, "org", "model"),
		IsLinked:         false,
	}

	err = LinkModel(mInfo)
	if err == nil {
		t.Errorf("expected error from LinkModel when snapshots directory is missing, got nil")
	}
}

// TestUnlinkModelNoMetadata tests that UnlinkModel does nothing (and returns nil) when no metadata file exists.
func TestUnlinkModelNoMetadata(t *testing.T) {
	targetDir, err := ioutil.TempDir("", "target")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(targetDir)

	// Create a target directory without a metadata file.
	modelPath := filepath.Join(targetDir, "org", "model")
	if err := os.MkdirAll(modelPath, 0755); err != nil {
		t.Fatalf("failed to create target directory: %v", err)
	}

	mInfo := ModelInfo{
		CacheDirName:     "models--org--model",
		OrganizationName: "org",
		ModelName:        "model",
		TargetPath:       modelPath,
		IsLinked:         false,
	}

	// Expect no error even though metadata file is missing.
	if err := UnlinkModel(mInfo); err != nil {
		t.Errorf("expected no error from UnlinkModel when metadata is missing, got %v", err)
	}
}
