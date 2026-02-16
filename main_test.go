package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
