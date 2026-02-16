package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// helper: create a fake repo root with .git/info directory
func setupRepoRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// helper: write content to a file, creating parent dirs
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// helper: read file content, fatal on error
func readFileContent(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

// helper: assert file exists with expected content
func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	got := readFileContent(t, path)
	if got != expected {
		t.Errorf("%s: expected content %q, got %q", filepath.Base(path), expected, got)
	}
}

// helper: assert path does not exist
func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected %s to not exist, but it does", path)
	}
}

// helper: assert path exists
func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func TestFilterItems(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "filters deletion marker",
			input:    []string{"file1", deletionMarker, "file2"},
			expected: []string{"file1", "file2"},
		},
		{
			name:     "filters branches directory",
			input:    []string{"file1", branchesDir, "file2"},
			expected: []string{"file1", "file2"},
		},
		{
			name:     "filters both special items",
			input:    []string{"file1", deletionMarker, branchesDir, "file2"},
			expected: []string{"file1", "file2"},
		},
		{
			name:     "no filtering needed",
			input:    []string{"file1", "file2", "file3"},
			expected: []string{"file1", "file2", "file3"},
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterItems(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d items, got %d", len(tt.expected), len(result))
			}
			for i, item := range result {
				if item != tt.expected[i] {
					t.Errorf("expected %s at index %d, got %s", tt.expected[i], i, item)
				}
			}
		})
	}
}

func TestReadExcludeFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test repository structure
	gitInfoDir := filepath.Join(tempDir, ".git", "info")
	if err := os.MkdirAll(gitInfoDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create some test files that exist
	testFile1 := filepath.Join(tempDir, "test1.txt")
	testFile2 := filepath.Join(tempDir, "test2.txt")
	if err := os.WriteFile(testFile1, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(testFile2, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create exclude file
	excludePath := filepath.Join(gitInfoDir, "exclude")
	excludeContent := `# Comment line
test1.txt

test2.txt
nonexistent.txt
*.pattern
test3.txt/
`
	if err := os.WriteFile(excludePath, []byte(excludeContent), 0644); err != nil {
		t.Fatal(err)
	}

	items, err := readExcludeFile(tempDir)
	if err != nil {
		t.Fatalf("readExcludeFile failed: %v", err)
	}

	// Should only include existing files, no patterns, no comments
	expected := []string{"test1.txt", "test2.txt"}
	if len(items) != len(expected) {
		t.Fatalf("expected %d items, got %d: %v", len(expected), len(items), items)
	}

	for i, item := range items {
		if item != expected[i] {
			t.Errorf("expected %s at index %d, got %s", expected[i], i, item)
		}
	}
}

func TestAddToExclude(t *testing.T) {
	tempDir := t.TempDir()

	// Test adding first item
	if err := addToExclude(tempDir, "file1.txt"); err != nil {
		t.Fatalf("failed to add first item: %v", err)
	}

	// Verify file was created and contains item
	excludePath := filepath.Join(tempDir, ".git", "info", "exclude")
	content, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("failed to read exclude file: %v", err)
	}

	if !strings.Contains(string(content), "file1.txt") {
		t.Errorf("exclude file doesn't contain added item")
	}

	// Test adding duplicate (should not duplicate)
	if err := addToExclude(tempDir, "file1.txt"); err != nil {
		t.Fatalf("failed to add duplicate item: %v", err)
	}

	content, err = os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("failed to read exclude file: %v", err)
	}

	count := strings.Count(string(content), "file1.txt")
	if count != 1 {
		t.Errorf("expected 1 occurrence of file1.txt, got %d", count)
	}

	// Test adding second item
	if err := addToExclude(tempDir, "file2.txt"); err != nil {
		t.Fatalf("failed to add second item: %v", err)
	}

	content, err = os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("failed to read exclude file: %v", err)
	}

	if !strings.Contains(string(content), "file2.txt") {
		t.Errorf("exclude file doesn't contain second item")
	}
}

func TestCopyFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create source file
	srcPath := filepath.Join(tempDir, "source.txt")
	content := "test content"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy file
	dstPath := filepath.Join(tempDir, "dest.txt")
	if err := copyFile(srcPath, dstPath); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}

	// Verify content
	dstContent, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}

	if string(dstContent) != content {
		t.Errorf("expected content %q, got %q", content, string(dstContent))
	}

	// Verify permissions
	srcInfo, _ := os.Stat(srcPath)
	dstInfo, _ := os.Stat(dstPath)
	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("permissions not copied correctly")
	}
}

func TestCopyDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tempDir, "source")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	file1 := filepath.Join(srcDir, "file1.txt")
	file2 := filepath.Join(srcDir, "subdir", "file2.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	// Copy directory
	dstDir := filepath.Join(tempDir, "dest")
	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Verify structure
	dstFile1 := filepath.Join(dstDir, "file1.txt")
	dstFile2 := filepath.Join(dstDir, "subdir", "file2.txt")

	content1, err := os.ReadFile(dstFile1)
	if err != nil {
		t.Fatalf("failed to read copied file1: %v", err)
	}
	if string(content1) != "content1" {
		t.Errorf("file1 content mismatch")
	}

	content2, err := os.ReadFile(dstFile2)
	if err != nil {
		t.Fatalf("failed to read copied file2: %v", err)
	}
	if string(content2) != "content2" {
		t.Errorf("file2 content mismatch")
	}
}

func TestListDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files and directories
	if err := os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "file2.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tempDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	items, err := listDir(tempDir)
	if err != nil {
		t.Fatalf("listDir failed: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Test non-existent directory
	items, err = listDir(filepath.Join(tempDir, "nonexistent"))
	if err != nil {
		t.Fatalf("listDir should not error on non-existent dir: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice for non-existent dir, got %d items", len(items))
	}
}

func TestReadExcludeFile_NoExcludeFile(t *testing.T) {
	repoRoot := setupRepoRoot(t)

	items, err := readExcludeFile(repoRoot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty slice, got %d items", len(items))
	}
}

func TestCopyPath_File(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	writeFile(t, src, "hello")

	if err := copyPath(src, dst); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, dst, "hello")
}

func TestCopyPath_Dir(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "srcdir")
	writeFile(t, filepath.Join(srcDir, "a.txt"), "aaa")
	dstDir := filepath.Join(dir, "dstdir")

	if err := copyPath(srcDir, dstDir); err != nil {
		t.Fatal(err)
	}
	assertFileContent(t, filepath.Join(dstDir, "a.txt"), "aaa")
}

func TestSyncIn(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	// Populate storage with files and a directory
	writeFile(t, filepath.Join(store, "notes.md"), "my notes")
	writeFile(t, filepath.Join(store, "config", "settings.json"), `{"key":"val"}`)

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncIn(cfg); err != nil {
		t.Fatalf("syncIn failed: %v", err)
	}

	// Files should be copied to repo root
	assertFileContent(t, filepath.Join(repoRoot, "notes.md"), "my notes")
	assertFileContent(t, filepath.Join(repoRoot, "config", "settings.json"), `{"key":"val"}`)

	// Files should be in .git/info/exclude
	excludeContent := readFileContent(t, filepath.Join(repoRoot, ".git", "info", "exclude"))
	if !strings.Contains(excludeContent, "notes.md") {
		t.Error("notes.md not in exclude file")
	}
	if !strings.Contains(excludeContent, "config") {
		t.Error("config not in exclude file")
	}
}

func TestSyncIn_EmptyStorage(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncIn(cfg); err != nil {
		t.Fatalf("syncIn failed on empty storage: %v", err)
	}
}

func TestSyncIn_NonexistentStorage(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := filepath.Join(t.TempDir(), "does-not-exist")

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncIn(cfg); err != nil {
		t.Fatalf("syncIn failed on nonexistent storage: %v", err)
	}
}

func TestSyncIn_FiltersSpecialItems(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	// Put special items in storage that should be filtered
	writeFile(t, filepath.Join(store, deletionMarker), "12345")
	if err := os.Mkdir(filepath.Join(store, branchesDir), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(store, "real-file.txt"), "data")

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncIn(cfg); err != nil {
		t.Fatal(err)
	}

	// real-file should be copied, special items should not
	assertFileContent(t, filepath.Join(repoRoot, "real-file.txt"), "data")
	assertNotExists(t, filepath.Join(repoRoot, deletionMarker))
	assertNotExists(t, filepath.Join(repoRoot, branchesDir))
}

func TestSyncOut(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	// Create files in repo root
	writeFile(t, filepath.Join(repoRoot, "notes.md"), "updated notes")
	writeFile(t, filepath.Join(repoRoot, "scratch.txt"), "scratch")

	// Write exclude file listing both
	excludePath := filepath.Join(repoRoot, ".git", "info", "exclude")
	writeFile(t, excludePath, "notes.md\nscratch.txt\n")

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncOut(cfg); err != nil {
		t.Fatalf("syncOut failed: %v", err)
	}

	// Both files should be in storage
	assertFileContent(t, filepath.Join(store, "notes.md"), "updated notes")
	assertFileContent(t, filepath.Join(store, "scratch.txt"), "scratch")
}

func TestSyncOut_RemovesStaleItems(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	// Pre-populate storage with a file that's no longer in exclude
	writeFile(t, filepath.Join(store, "old-file.txt"), "stale")
	writeFile(t, filepath.Join(store, "current.txt"), "old content")

	// Repo has current.txt, exclude only lists current.txt
	writeFile(t, filepath.Join(repoRoot, "current.txt"), "new content")
	writeFile(t, filepath.Join(repoRoot, ".git", "info", "exclude"), "current.txt\n")

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncOut(cfg); err != nil {
		t.Fatal(err)
	}

	// current.txt updated in storage, old-file.txt removed
	assertFileContent(t, filepath.Join(store, "current.txt"), "new content")
	assertNotExists(t, filepath.Join(store, "old-file.txt"))
}

func TestSyncOut_PreservesSpecialItems(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	// Storage has branches dir and deletion marker — these must survive cleanup
	if err := os.Mkdir(filepath.Join(store, branchesDir), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(store, deletionMarker), "12345")

	// Empty exclude file — nothing managed
	writeFile(t, filepath.Join(repoRoot, ".git", "info", "exclude"), "")

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := syncOut(cfg); err != nil {
		t.Fatal(err)
	}

	// Special items must not be deleted
	assertExists(t, filepath.Join(store, branchesDir))
	assertExists(t, filepath.Join(store, deletionMarker))
}

func TestSyncOut_NoExcludeFile(t *testing.T) {
	repoRoot := setupRepoRoot(t)
	store := t.TempDir()

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	// Should not error when no exclude file exists
	if err := syncOut(cfg); err != nil {
		t.Fatalf("syncOut failed: %v", err)
	}
}

func TestInitializeBranchStorage_DefaultBranchNoop(t *testing.T) {
	store := t.TempDir()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := initializeBranchStorage(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestInitializeBranchStorage_ExistingStorageNoop(t *testing.T) {
	store := t.TempDir()
	branchStore := filepath.Join(store, branchesDir, "feature")
	if err := os.MkdirAll(branchStore, 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(branchStore, "existing.txt"), "keep me")

	cfg := &Config{
		CurrentBranch: "feature",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: branchStore,
	}

	if err := initializeBranchStorage(cfg); err != nil {
		t.Fatal(err)
	}

	// Existing content should be untouched
	assertFileContent(t, filepath.Join(branchStore, "existing.txt"), "keep me")
}

func TestInitializeBranchStorage_CopiesFromDefault(t *testing.T) {
	store := t.TempDir()

	// Populate default branch storage
	writeFile(t, filepath.Join(store, "notes.md"), "default notes")
	writeFile(t, filepath.Join(store, "config.json"), `{"a":1}`)
	// Add special items that should NOT be copied
	if err := os.Mkdir(filepath.Join(store, branchesDir), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(store, deletionMarker), "99999")

	branchStore := filepath.Join(store, branchesDir, "feature-x")

	cfg := &Config{
		CurrentBranch: "feature-x",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: branchStore,
	}

	if err := initializeBranchStorage(cfg); err != nil {
		t.Fatal(err)
	}

	// Should have copied normal files
	assertFileContent(t, filepath.Join(branchStore, "notes.md"), "default notes")
	assertFileContent(t, filepath.Join(branchStore, "config.json"), `{"a":1}`)
	// Should NOT have copied special items
	assertNotExists(t, filepath.Join(branchStore, branchesDir))
	assertNotExists(t, filepath.Join(branchStore, deletionMarker))
}

func TestInitializeBranchStorage_NoDefaultStorage(t *testing.T) {
	// StoreBase doesn't exist at all — should just create empty branch dir
	store := filepath.Join(t.TempDir(), "nonexistent-repo")
	branchStore := filepath.Join(store, branchesDir, "feature")

	cfg := &Config{
		CurrentBranch: "feature",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: branchStore,
	}

	if err := initializeBranchStorage(cfg); err != nil {
		t.Fatal(err)
	}

	assertExists(t, branchStore)
	items, _ := listDir(branchStore)
	if len(items) != 0 {
		t.Errorf("expected empty branch store, got %v", items)
	}
}

func TestCleanupDeletedBranches_NoBranchesDir(t *testing.T) {
	store := t.TempDir()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	// Should return nil when branches/ doesn't exist
	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupDeletedBranches_SkipsCurrentBranch(t *testing.T) {
	store := t.TempDir()
	branchesPath := filepath.Join(store, branchesDir)

	// Create storage for "feature" (which is the current branch)
	writeFile(t, filepath.Join(branchesPath, "feature", "file.txt"), "data")

	// Mock: "feature" does NOT exist in git — but it's current, so skip it
	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return map[string]bool{"main": true}, nil
	}
	defer func() { getAllBranchesFunc = orig }()

	cfg := &Config{
		CurrentBranch: "feature",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: filepath.Join(branchesPath, "feature"),
	}

	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}

	// Branch storage must still exist
	assertExists(t, filepath.Join(branchesPath, "feature", "file.txt"))
}

func TestCleanupDeletedBranches_LeavesExistingBranches(t *testing.T) {
	store := t.TempDir()
	branchesPath := filepath.Join(store, branchesDir)

	writeFile(t, filepath.Join(branchesPath, "feature", "file.txt"), "data")

	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return map[string]bool{"main": true, "feature": true}, nil
	}
	defer func() { getAllBranchesFunc = orig }()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}

	assertExists(t, filepath.Join(branchesPath, "feature", "file.txt"))
}

func TestCleanupDeletedBranches_RemovesStaleMarkerForExistingBranch(t *testing.T) {
	store := t.TempDir()
	branchesPath := filepath.Join(store, branchesDir)

	// Branch "feature" has a stale deletion marker but the branch exists in git
	writeFile(t, filepath.Join(branchesPath, "feature", "file.txt"), "data")
	writeFile(t, filepath.Join(branchesPath, "feature", deletionMarker), "12345")

	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return map[string]bool{"main": true, "feature": true}, nil
	}
	defer func() { getAllBranchesFunc = orig }()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}

	// Marker should be removed, files kept
	assertExists(t, filepath.Join(branchesPath, "feature", "file.txt"))
	assertNotExists(t, filepath.Join(branchesPath, "feature", deletionMarker))
}

func TestCleanupDeletedBranches_CreatesMarkerForDeletedBranch(t *testing.T) {
	store := t.TempDir()
	branchesPath := filepath.Join(store, branchesDir)

	writeFile(t, filepath.Join(branchesPath, "gone-branch", "file.txt"), "data")

	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return map[string]bool{"main": true}, nil
	}
	defer func() { getAllBranchesFunc = orig }()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}

	// Marker should be created, files still present (within grace period)
	markerPath := filepath.Join(branchesPath, "gone-branch", deletionMarker)
	assertExists(t, markerPath)
	assertExists(t, filepath.Join(branchesPath, "gone-branch", "file.txt"))

	// Marker should contain a recent unix timestamp
	content := readFileContent(t, markerPath)
	ts, err := strconv.ParseInt(strings.TrimSpace(content), 10, 64)
	if err != nil {
		t.Fatalf("marker is not a valid timestamp: %v", err)
	}
	if time.Since(time.Unix(ts, 0)) > 5*time.Second {
		t.Error("marker timestamp is not recent")
	}
}

func TestCleanupDeletedBranches_DeletesAfterGracePeriod(t *testing.T) {
	store := t.TempDir()
	branchesPath := filepath.Join(store, branchesDir)

	// Create branch storage with an expired deletion marker (8 days ago)
	writeFile(t, filepath.Join(branchesPath, "old-branch", "file.txt"), "data")
	expiredTs := time.Now().Add(-8 * 24 * time.Hour).Unix()
	writeFile(t, filepath.Join(branchesPath, "old-branch", deletionMarker),
		fmt.Sprintf("%d", expiredTs))

	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return map[string]bool{"main": true}, nil
	}
	defer func() { getAllBranchesFunc = orig }()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}

	// Entire branch directory should be removed
	assertNotExists(t, filepath.Join(branchesPath, "old-branch"))
}

func TestCleanupDeletedBranches_KeepsDuringGracePeriod(t *testing.T) {
	store := t.TempDir()
	branchesPath := filepath.Join(store, branchesDir)

	// Create branch storage with a recent deletion marker (1 day ago)
	writeFile(t, filepath.Join(branchesPath, "recent-branch", "file.txt"), "data")
	recentTs := time.Now().Add(-1 * 24 * time.Hour).Unix()
	writeFile(t, filepath.Join(branchesPath, "recent-branch", deletionMarker),
		fmt.Sprintf("%d", recentTs))

	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return map[string]bool{"main": true}, nil
	}
	defer func() { getAllBranchesFunc = orig }()

	cfg := &Config{
		CurrentBranch: "main",
		DefaultBranch: "main",
		StoreBase:     store,
		StoreLocation: store,
	}

	if err := cleanupDeletedBranches(cfg); err != nil {
		t.Fatal(err)
	}

	// Branch should still exist — within grace period
	assertExists(t, filepath.Join(branchesPath, "recent-branch", "file.txt"))
	assertExists(t, filepath.Join(branchesPath, "recent-branch", deletionMarker))
}
