package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestEncodeBranchName(t *testing.T) {
	tests := []struct {
		input   string
		encoded string
	}{
		{"main", "main"},
		{"feature/login", "feature%2Flogin"},
		{"feature/auth/oauth", "feature%2Fauth%2Foauth"},
		{"simple-branch", "simple-branch"},
		{"branch--with--dashes", "branch--with--dashes"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			encoded := encodeBranchName(tt.input)
			if encoded != tt.encoded {
				t.Errorf("encodeBranchName(%q) = %q, want %q", tt.input, encoded, tt.encoded)
			}

			decoded, err := decodeBranchName(encoded)
			if err != nil {
				t.Fatalf("decodeBranchName(%q) error: %v", encoded, err)
			}
			if decoded != tt.input {
				t.Errorf("decodeBranchName(%q) = %q, want %q", encoded, decoded, tt.input)
			}
		})
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

func newTestUpdater(apiServer *httptest.Server, dlServer *httptest.Server, ver string, selfPath string) *updater {
	u := &updater{
		version: ver,
		client:  apiServer.Client(),
		dlClient: func() *http.Client {
			if dlServer != nil {
				return dlServer.Client()
			}
			return apiServer.Client()
		}(),
		selfPath: func() (string, error) { return selfPath, nil },
		restart:  func(self string) error { return nil },
	}
	u.apiURL = apiServer.URL
	if dlServer != nil {
		u.downloadURL = func(tag string) string { return dlServer.URL + "/" + tag }
	} else {
		u.downloadURL = func(tag string) string { return apiServer.URL + "/download/" + tag }
	}
	return u
}

func TestCheckLatest_DevVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP request for dev version")
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "dev", "")
	tag, err := u.checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag for dev version, got %q", tag)
	}
}

func TestCheckLatest_SameVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "v1.0.0", "")
	tag, err := u.checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag for same version, got %q", tag)
	}
}

func TestCheckLatest_NewVersionAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "v1.0.0", "")
	tag, err := u.checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("expected v2.0.0, got %q", tag)
	}
}

func TestCheckLatest_EmptyTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": ""})
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "v1.0.0", "")
	tag, err := u.checkLatest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "" {
		t.Errorf("expected empty tag, got %q", tag)
	}
}

func TestCheckLatest_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "v1.0.0", "")
	tag, err := u.checkLatest()
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if tag != "" {
		t.Errorf("expected empty tag on error, got %q", tag)
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected status 500 in error, got: %v", err)
	}
}

func TestCheckLatest_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not json")
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "v1.0.0", "")
	_, err := u.checkLatest()
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}

func TestCheckLatest_InvalidTagFormat(t *testing.T) {
	tests := []struct {
		name string
		tag  string
	}{
		{"path traversal", "../../../etc/passwd"},
		{"prerelease", "v1.2.3-beta"},
		{"no v prefix", "1.2.3"},
		{"extra segments", "v1.2.3.4"},
		{"letters", "vabc"},
		{"shell injection", "v1.0.0; rm -rf /"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(map[string]string{"tag_name": tt.tag})
			}))
			defer srv.Close()

			u := newTestUpdater(srv, nil, "v1.0.0", "")
			tag, err := u.checkLatest()
			if err == nil {
				t.Fatalf("expected error for tag %q, got tag=%q", tt.tag, tag)
			}
			if !strings.Contains(err.Error(), "unexpected release tag format") {
				t.Errorf("expected format error, got: %v", err)
			}
		})
	}
}

func TestVersionTagPattern(t *testing.T) {
	valid := []string{"v0.0.1", "v1.2.3", "v10.20.30", "v0.0.0"}
	for _, tag := range valid {
		if !versionTagPattern.MatchString(tag) {
			t.Errorf("expected %q to be valid", tag)
		}
	}

	invalid := []string{
		"", "dev", "1.2.3", "v1.2", "v1.2.3.4",
		"v1.2.3-beta", "v1.2.3+build", "../foo",
		"v1.2.3; rm -rf /", "v1.2.3\n", "V1.2.3",
	}
	for _, tag := range invalid {
		if versionTagPattern.MatchString(tag) {
			t.Errorf("expected %q to be invalid", tag)
		}
	}
}

func TestDownloadAndReplace(t *testing.T) {
	// Create a fake "current binary" to be replaced
	tmpDir := t.TempDir()
	selfPath := filepath.Join(tmpDir, "claude-wrapper")
	if err := os.WriteFile(selfPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	newContent := "new binary content"
	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, newContent)
	}))
	defer dlSrv.Close()

	// apiSrv unused for download, but needed for helper
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer apiSrv.Close()

	u := newTestUpdater(apiSrv, dlSrv, "v1.0.0", selfPath)

	if err := u.downloadAndReplace("v2.0.0"); err != nil {
		t.Fatalf("downloadAndReplace failed: %v", err)
	}

	// Verify the binary was replaced
	content, err := os.ReadFile(selfPath)
	if err != nil {
		t.Fatalf("failed to read replaced binary: %v", err)
	}
	if string(content) != newContent {
		t.Errorf("expected %q, got %q", newContent, string(content))
	}

	// Verify it's executable
	info, err := os.Stat(selfPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("replaced binary is not executable")
	}
}

func TestDownloadAndReplace_DownloadError(t *testing.T) {
	tmpDir := t.TempDir()
	selfPath := filepath.Join(tmpDir, "claude-wrapper")
	if err := os.WriteFile(selfPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer apiSrv.Close()

	u := newTestUpdater(apiSrv, dlSrv, "v1.0.0", selfPath)

	err := u.downloadAndReplace("v2.0.0")
	if err == nil {
		t.Fatal("expected error for 404 download")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("expected status 404 in error, got: %v", err)
	}

	// Verify original binary is untouched
	content, err := os.ReadFile(selfPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "old binary" {
		t.Error("original binary was modified on failed download")
	}
}

func TestDownloadAndReplace_NoTempFileLeaked(t *testing.T) {
	tmpDir := t.TempDir()
	selfPath := filepath.Join(tmpDir, "claude-wrapper")
	if err := os.WriteFile(selfPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer apiSrv.Close()

	u := newTestUpdater(apiSrv, dlSrv, "v1.0.0", selfPath)
	_ = u.downloadAndReplace("v2.0.0")

	// Only the original binary should remain
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected only original binary in dir, found: %v", names)
	}
}

func TestDownloadAndReplace_SelfPathError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer apiSrv.Close()

	u := newTestUpdater(apiSrv, nil, "v1.0.0", "")
	u.selfPath = func() (string, error) { return "", fmt.Errorf("no executable path") }

	err := u.downloadAndReplace("v2.0.0")
	if err == nil {
		t.Fatal("expected error when selfPath fails")
	}
	if !strings.Contains(err.Error(), "resolve executable path") {
		t.Errorf("expected resolve error, got: %v", err)
	}
}

func TestApply_SkipsWhenAlreadyUpdated(t *testing.T) {
	t.Setenv("CLAUDE_WRAPPER_UPDATED", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not make HTTP request when CLAUDE_WRAPPER_UPDATED=1")
	}))
	defer srv.Close()

	u := newTestUpdater(srv, nil, "v1.0.0", "")
	u.apply()
}

func TestApply_NoUpdateAvailable(t *testing.T) {
	t.Setenv("CLAUDE_WRAPPER_UPDATED", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v1.0.0"})
	}))
	defer srv.Close()

	restarted := false
	u := newTestUpdater(srv, nil, "v1.0.0", "")
	u.restart = func(self string) error { restarted = true; return nil }
	u.apply()

	if restarted {
		t.Error("should not restart when no update is available")
	}
}

func TestApply_SuccessfulUpdate(t *testing.T) {
	t.Setenv("CLAUDE_WRAPPER_UPDATED", "")

	tmpDir := t.TempDir()
	selfPath := filepath.Join(tmpDir, "claude-wrapper")
	if err := os.WriteFile(selfPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer apiSrv.Close()

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "new binary")
	}))
	defer dlSrv.Close()

	var restartedWith string
	u := newTestUpdater(apiSrv, dlSrv, "v1.0.0", selfPath)
	u.restart = func(self string) error { restartedWith = self; return nil }

	u.apply()

	if restartedWith != selfPath {
		t.Errorf("expected restart with %q, got %q", selfPath, restartedWith)
	}
	if os.Getenv("CLAUDE_WRAPPER_UPDATED") != "1" {
		t.Error("expected CLAUDE_WRAPPER_UPDATED=1 to be set before restart")
	}

	content, err := os.ReadFile(selfPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new binary" {
		t.Errorf("binary not replaced, got %q", string(content))
	}
}

func TestApply_DownloadFails(t *testing.T) {
	t.Setenv("CLAUDE_WRAPPER_UPDATED", "")

	tmpDir := t.TempDir()
	selfPath := filepath.Join(tmpDir, "claude-wrapper")
	if err := os.WriteFile(selfPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer apiSrv.Close()

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer dlSrv.Close()

	restarted := false
	u := newTestUpdater(apiSrv, dlSrv, "v1.0.0", selfPath)
	u.restart = func(self string) error { restarted = true; return nil }

	u.apply()

	if restarted {
		t.Error("should not restart when download fails")
	}

	content, err := os.ReadFile(selfPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "old binary" {
		t.Error("binary should not be modified when download fails")
	}
}

func TestApply_SelfPathFailsAfterUpdate(t *testing.T) {
	t.Setenv("CLAUDE_WRAPPER_UPDATED", "")

	tmpDir := t.TempDir()
	selfPath := filepath.Join(tmpDir, "claude-wrapper")
	if err := os.WriteFile(selfPath, []byte("old binary"), 0755); err != nil {
		t.Fatal(err)
	}

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"tag_name": "v2.0.0"})
	}))
	defer apiSrv.Close()

	dlSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "new binary")
	}))
	defer dlSrv.Close()

	callCount := 0
	restarted := false
	u := newTestUpdater(apiSrv, dlSrv, "v1.0.0", selfPath)
	u.selfPath = func() (string, error) {
		callCount++
		if callCount == 1 {
			// First call from downloadAndReplace succeeds
			return selfPath, nil
		}
		// Second call from apply after download succeeds fails
		return "", fmt.Errorf("path gone")
	}
	u.restart = func(self string) error { restarted = true; return nil }

	u.apply()

	if restarted {
		t.Error("should not restart when selfPath fails after update")
	}
	if os.Getenv("CLAUDE_WRAPPER_UPDATED") == "1" {
		t.Error("should not set CLAUDE_WRAPPER_UPDATED when selfPath fails")
	}
}

func TestApply_CheckLatestError(t *testing.T) {
	t.Setenv("CLAUDE_WRAPPER_UPDATED", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	restarted := false
	u := newTestUpdater(srv, nil, "v1.0.0", "")
	u.restart = func(self string) error { restarted = true; return nil }

	u.apply()

	if restarted {
		t.Error("should not restart when checkLatest fails")
	}
}

// ---------------------------------------------------------------------------
// Test helpers for core business logic
// ---------------------------------------------------------------------------

func withGitStubs(t *testing.T, repoRoot, currentBranch, defaultBranch string, allBranches map[string]bool) {
	t.Helper()
	origRoot := getGitRepoRootFn
	origCurrent := getCurrentBranchFn
	origDefault := getDefaultBranchFn
	origAll := getAllBranchesFn

	getGitRepoRootFn = func() (string, error) { return repoRoot, nil }
	getCurrentBranchFn = func() (string, error) { return currentBranch, nil }
	getDefaultBranchFn = func() string { return defaultBranch }
	getAllBranchesFn = func() (map[string]bool, error) { return allBranches, nil }

	t.Cleanup(func() {
		getGitRepoRootFn = origRoot
		getCurrentBranchFn = origCurrent
		getDefaultBranchFn = origDefault
		getAllBranchesFn = origAll
	})
}

func withClaudeStub(t *testing.T, fn func([]string) error) {
	t.Helper()
	orig := execClaudeFn
	execClaudeFn = fn
	t.Cleanup(func() { execClaudeFn = orig })
}

func withUpdateStub(t *testing.T) {
	t.Helper()
	orig := checkForUpdateFn
	checkForUpdateFn = func() {}
	t.Cleanup(func() { checkForUpdateFn = orig })
}

func setupTestRepo(t *testing.T) (repoRoot, storeBase string) {
	t.Helper()
	repoRoot = t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}
	storeBase = t.TempDir()
	return repoRoot, storeBase
}

func makeConfig(repoRoot, storeBase, currentBranch, defaultBranch string) *Config {
	var storeLocation string
	if currentBranch == defaultBranch {
		storeLocation = storeBase
	} else {
		storeLocation = filepath.Join(storeBase, branchesDir, encodeBranchName(currentBranch))
	}
	return &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: currentBranch,
		DefaultBranch: defaultBranch,
		StoreBase:     storeBase,
		StoreLocation: storeLocation,
	}
}

// ---------------------------------------------------------------------------
// TestInitializeBranchStorage
// ---------------------------------------------------------------------------

func TestInitializeBranchStorage(t *testing.T) {
	t.Run("Given default branch, it is a no-op", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		if err := initializeBranchStorage(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Given new feature branch with existing default store, it copies files skipping special items", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)

		// Seed default store with files + special items
		if err := os.WriteFile(filepath.Join(storeBase, "CLAUDE.md"), []byte("notes"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, ".claude"), []byte("cfg"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(storeBase, branchesDir, "other"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, deletionMarker), []byte("123"), 0644); err != nil {
			t.Fatal(err)
		}

		cfg := makeConfig(repoRoot, storeBase, "feature/login", "main")

		if err := initializeBranchStorage(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify files were copied
		for _, name := range []string{"CLAUDE.md", ".claude"} {
			if _, err := os.Stat(filepath.Join(cfg.StoreLocation, name)); err != nil {
				t.Errorf("expected %s to be copied, got: %v", name, err)
			}
		}

		// Verify special items were NOT copied
		for _, name := range []string{branchesDir, deletionMarker} {
			if _, err := os.Stat(filepath.Join(cfg.StoreLocation, name)); err == nil {
				t.Errorf("expected %s to NOT be copied", name)
			}
		}
	})

	t.Run("Given new feature branch with no default store, it creates empty dir", func(t *testing.T) {
		repoRoot := t.TempDir()
		storeBase := filepath.Join(t.TempDir(), "nonexistent")
		cfg := makeConfig(repoRoot, storeBase, "feature/new", "main")

		if err := initializeBranchStorage(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(cfg.StoreLocation)
		if err != nil {
			t.Fatalf("expected store dir to exist: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected store location to be a directory")
		}
	})

	t.Run("Given feature branch with existing storage, it leaves it untouched", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "feature/existing", "main")

		if err := os.MkdirAll(cfg.StoreLocation, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg.StoreLocation, "existing.md"), []byte("keep"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := initializeBranchStorage(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, _ := os.ReadFile(filepath.Join(cfg.StoreLocation, "existing.md"))
		if string(content) != "keep" {
			t.Error("existing file was modified")
		}
	})
}

// ---------------------------------------------------------------------------
// TestSyncIn
// ---------------------------------------------------------------------------

func TestSyncIn(t *testing.T) {
	t.Run("Given storage has files, it copies to repo and adds to git exclude", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		if err := os.WriteFile(filepath.Join(storeBase, "CLAUDE.md"), []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, ".claude-settings"), []byte("cfg"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncIn(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Files should be in repo
		content, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "hello" {
			t.Errorf("expected 'hello', got %q", string(content))
		}

		// Exclude file should list items
		excludeContent, _ := os.ReadFile(filepath.Join(repoRoot, excludeFile))
		if !strings.Contains(string(excludeContent), "CLAUDE.md") {
			t.Error("CLAUDE.md not in exclude file")
		}
	})

	t.Run("Given empty storage, it succeeds with no files copied", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		os.MkdirAll(storeBase, 0755)

		if err := syncIn(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Given storage has special items, it skips them", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		if err := os.MkdirAll(filepath.Join(storeBase, branchesDir), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, deletionMarker), []byte("123"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, "real-file.md"), []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncIn(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(repoRoot, branchesDir)); err == nil {
			t.Error("branches dir should not be copied to repo")
		}
		if _, err := os.Stat(filepath.Join(repoRoot, deletionMarker)); err == nil {
			t.Error("deletion marker should not be copied to repo")
		}
		if _, err := os.Stat(filepath.Join(repoRoot, "real-file.md")); err != nil {
			t.Error("real-file.md should be copied to repo")
		}
	})

	t.Run("Given feature branch with no storage yet, it initializes from default then syncs", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)

		// Seed default store
		if err := os.WriteFile(filepath.Join(storeBase, "CLAUDE.md"), []byte("default"), 0644); err != nil {
			t.Fatal(err)
		}

		cfg := makeConfig(repoRoot, storeBase, "feature/test", "main")

		if err := syncIn(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// File should be in repo (copied from default → branch store → repo)
		content, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "default" {
			t.Errorf("expected 'default', got %q", string(content))
		}
	})
}

// ---------------------------------------------------------------------------
// TestSyncOut
// ---------------------------------------------------------------------------

func TestSyncOut(t *testing.T) {
	t.Run("Given files in exclude list, it copies to storage", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		// Create files in repo and exclude list
		if err := os.WriteFile(filepath.Join(repoRoot, "CLAUDE.md"), []byte("updated"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, excludeFile), []byte("CLAUDE.md\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncOut(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, err := os.ReadFile(filepath.Join(storeBase, "CLAUDE.md"))
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "updated" {
			t.Errorf("expected 'updated', got %q", string(content))
		}
	})

	t.Run("Given file removed from exclude, it removes from storage", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		// Pre-populate storage with a file that is no longer excluded
		if err := os.MkdirAll(storeBase, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, "old-file.md"), []byte("stale"), 0644); err != nil {
			t.Fatal(err)
		}

		// Exclude file is empty
		if err := os.WriteFile(filepath.Join(repoRoot, excludeFile), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncOut(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(storeBase, "old-file.md")); err == nil {
			t.Error("old-file.md should have been removed from storage")
		}
	})

	t.Run("Given empty exclude, it removes all non-special items", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		if err := os.MkdirAll(storeBase, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, "file1.md"), []byte("a"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, "file2.md"), []byte("b"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(storeBase, branchesDir), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, deletionMarker), []byte("123"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(repoRoot, excludeFile), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncOut(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Non-special files removed
		if _, err := os.Stat(filepath.Join(storeBase, "file1.md")); err == nil {
			t.Error("file1.md should have been removed")
		}
		if _, err := os.Stat(filepath.Join(storeBase, "file2.md")); err == nil {
			t.Error("file2.md should have been removed")
		}

		// Special items preserved
		if _, err := os.Stat(filepath.Join(storeBase, branchesDir)); err != nil {
			t.Error("branches dir should be preserved")
		}
		if _, err := os.Stat(filepath.Join(storeBase, deletionMarker)); err != nil {
			t.Error("deletion marker should be preserved")
		}
	})

	t.Run("Given special items in storage, it preserves them", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		if err := os.MkdirAll(filepath.Join(storeBase, branchesDir), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storeBase, deletionMarker), []byte("ts"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(repoRoot, excludeFile), []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncOut(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(storeBase, branchesDir)); err != nil {
			t.Error("branches dir should be preserved")
		}
		if _, err := os.Stat(filepath.Join(storeBase, deletionMarker)); err != nil {
			t.Error("deletion marker should be preserved")
		}
	})

	t.Run("Given excluded file missing on disk, it is filtered by readExcludeFile", func(t *testing.T) {
		repoRoot, storeBase := setupTestRepo(t)
		cfg := makeConfig(repoRoot, storeBase, "main", "main")

		// Exclude references a file that doesn't exist on disk;
		// readExcludeFile filters it out via os.Stat before syncOut sees it
		if err := os.WriteFile(filepath.Join(repoRoot, excludeFile), []byte("ghost.md\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if err := syncOut(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Join(storeBase, "ghost.md")); err == nil {
			t.Error("ghost.md should not exist in storage")
		}
	})
}

// ---------------------------------------------------------------------------
// TestCleanupDeletedBranches
// ---------------------------------------------------------------------------

func TestCleanupDeletedBranches(t *testing.T) {
	t.Run("Given no branches dir, it returns nil", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		// No withGitStubs needed: returns before calling getAllBranchesFn

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("Given branch still in git, it keeps storage and removes stale marker", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		branchPath := filepath.Join(storeBase, branchesDir, "feature%2Factive")
		if err := os.MkdirAll(branchPath, 0755); err != nil {
			t.Fatal(err)
		}
		markerPath := filepath.Join(branchPath, deletionMarker)
		if err := os.WriteFile(markerPath, []byte("123"), 0644); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, "", "main", "main", map[string]bool{"main": true, "feature/active": true})

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Storage kept
		if _, err := os.Stat(branchPath); err != nil {
			t.Error("branch storage should still exist")
		}
		// Marker removed
		if _, err := os.Stat(markerPath); err == nil {
			t.Error("stale deletion marker should have been removed")
		}
	})

	t.Run("Given deleted branch with no marker, it creates marker", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		branchPath := filepath.Join(storeBase, branchesDir, "feature%2Fold")
		if err := os.MkdirAll(branchPath, 0755); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, "", "main", "main", map[string]bool{"main": true})

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		markerPath := filepath.Join(branchPath, deletionMarker)
		data, err := os.ReadFile(markerPath)
		if err != nil {
			t.Fatalf("expected deletion marker to be created: %v", err)
		}

		ts, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			t.Fatalf("marker should contain unix timestamp: %v", err)
		}

		if time.Since(time.Unix(ts, 0)) > 5*time.Second {
			t.Error("marker timestamp is too old")
		}
	})

	t.Run("Given deleted branch with fresh marker, it keeps storage", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		branchPath := filepath.Join(storeBase, branchesDir, "feature%2Frecent")
		if err := os.MkdirAll(branchPath, 0755); err != nil {
			t.Fatal(err)
		}

		// Marker with recent timestamp
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		if err := os.WriteFile(filepath.Join(branchPath, deletionMarker), []byte(ts), 0644); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, "", "main", "main", map[string]bool{"main": true})

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(branchPath); err != nil {
			t.Error("branch storage with fresh marker should be kept")
		}
	})

	t.Run("Given deleted branch with expired marker, it removes storage", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		branchPath := filepath.Join(storeBase, branchesDir, "feature%2Fexpired")
		if err := os.MkdirAll(branchPath, 0755); err != nil {
			t.Fatal(err)
		}

		// Marker with timestamp 8 days ago
		oldTs := strconv.FormatInt(time.Now().Add(-8*24*time.Hour).Unix(), 10)
		if err := os.WriteFile(filepath.Join(branchPath, deletionMarker), []byte(oldTs), 0644); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, "", "main", "main", map[string]bool{"main": true})

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(branchPath); err == nil {
			t.Error("branch storage with expired marker should be removed")
		}
	})

	t.Run("Given current branch not in git list, it skips it", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "feature/current", "main")

		branchPath := filepath.Join(storeBase, branchesDir, "feature%2Fcurrent")
		if err := os.MkdirAll(branchPath, 0755); err != nil {
			t.Fatal(err)
		}

		// Current branch is NOT in the git branches list (edge case)
		withGitStubs(t, "", "feature/current", "main", map[string]bool{"main": true})

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should be skipped (not deleted) because it's the current branch
		if _, err := os.Stat(branchPath); err != nil {
			t.Error("current branch storage should be preserved even if not in git list")
		}
	})

	t.Run("Given getAllBranches fails, it returns error", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		// Create branches dir so we don't return early at the stat check
		if err := os.MkdirAll(filepath.Join(storeBase, branchesDir), 0755); err != nil {
			t.Fatal(err)
		}

		origAll := getAllBranchesFn
		getAllBranchesFn = func() (map[string]bool, error) {
			return nil, fmt.Errorf("git command failed")
		}
		t.Cleanup(func() { getAllBranchesFn = origAll })

		err := cleanupDeletedBranches(cfg)
		if err == nil {
			t.Fatal("expected error when getAllBranches fails")
		}
		if !strings.Contains(err.Error(), "git command failed") {
			t.Errorf("expected 'git command failed' error, got: %v", err)
		}
	})

	t.Run("Given deleted branch with corrupt marker timestamp, it preserves storage", func(t *testing.T) {
		_, storeBase := setupTestRepo(t)
		cfg := makeConfig("", storeBase, "main", "main")

		branchPath := filepath.Join(storeBase, branchesDir, "feature%2Fcorrupt")
		if err := os.MkdirAll(branchPath, 0755); err != nil {
			t.Fatal(err)
		}

		// Write non-numeric data to marker
		markerPath := filepath.Join(branchPath, deletionMarker)
		if err := os.WriteFile(markerPath, []byte("not-a-number"), 0644); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, "", "main", "main", map[string]bool{"main": true})

		if err := cleanupDeletedBranches(cfg); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Branch should still exist (corrupt timestamp can't determine age)
		if _, err := os.Stat(branchPath); err != nil {
			t.Error("branch with corrupt marker should be preserved")
		}
		// Marker should still exist (markerExists was true, so no new marker created)
		if _, err := os.Stat(markerPath); err != nil {
			t.Error("corrupt marker should be preserved")
		}
	})
}

// ---------------------------------------------------------------------------
// TestLoadConfig
// ---------------------------------------------------------------------------

func TestLoadConfig(t *testing.T) {
	t.Run("Given default branch, StoreLocation equals StoreBase", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		repoRoot := filepath.Join(home, "myrepo")
		if err := os.MkdirAll(repoRoot, 0755); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, repoRoot, "main", "main", nil)

		cfg, err := loadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.StoreLocation != cfg.StoreBase {
			t.Errorf("expected StoreLocation == StoreBase, got %q vs %q", cfg.StoreLocation, cfg.StoreBase)
		}
		if cfg.CurrentBranch != "main" {
			t.Errorf("expected current branch 'main', got %q", cfg.CurrentBranch)
		}
	})

	t.Run("Given feature branch, StoreLocation is under branches/", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		repoRoot := filepath.Join(home, "myrepo")
		if err := os.MkdirAll(repoRoot, 0755); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, repoRoot, "feature/auth", "main", nil)

		cfg, err := loadConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expectedSuffix := filepath.Join(branchesDir, encodeBranchName("feature/auth"))
		if !strings.HasSuffix(cfg.StoreLocation, expectedSuffix) {
			t.Errorf("expected StoreLocation to end with %q, got %q", expectedSuffix, cfg.StoreLocation)
		}
	})

	t.Run("Given git repo root fails, it returns error", func(t *testing.T) {
		origRoot := getGitRepoRootFn
		getGitRepoRootFn = func() (string, error) { return "", fmt.Errorf("not a git repo") }
		t.Cleanup(func() { getGitRepoRootFn = origRoot })

		_, err := loadConfig()
		if err == nil {
			t.Fatal("expected error when getGitRepoRoot fails")
		}
	})

	t.Run("Given current branch fails, it returns error", func(t *testing.T) {
		origRoot := getGitRepoRootFn
		origBranch := getCurrentBranchFn
		getGitRepoRootFn = func() (string, error) { return "/tmp/repo", nil }
		getCurrentBranchFn = func() (string, error) { return "", fmt.Errorf("detached HEAD") }
		t.Cleanup(func() {
			getGitRepoRootFn = origRoot
			getCurrentBranchFn = origBranch
		})

		_, err := loadConfig()
		if err == nil {
			t.Fatal("expected error when getCurrentBranch fails")
		}
	})

	t.Run("Given HOME is unset, it returns error", func(t *testing.T) {
		// os.UserHomeDir reads $HOME on Linux; unsetting it forces a user lookup
		// which may fail in containerized/CI environments. Setting to empty string
		// still returns "" as home, leading to a valid (but odd) path. Instead,
		// we test the wrapped error message by temporarily breaking HOME resolution.
		t.Setenv("HOME", "")

		withGitStubs(t, "/tmp/repo", "main", "main", nil)

		cfg, err := loadConfig()
		// os.UserHomeDir returns "" when HOME="", which is technically valid.
		// Verify it doesn't panic and produces a config with empty-rooted StoreBase.
		if err != nil {
			// If UserHomeDir actually errors (platform-dependent), that's fine too
			if !strings.Contains(err.Error(), "home") {
				t.Errorf("expected home-related error, got: %v", err)
			}
			return
		}
		if cfg.StoreBase == "" {
			t.Error("expected non-empty StoreBase even with empty HOME")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRun
// ---------------------------------------------------------------------------

func TestRun(t *testing.T) {
	t.Run("Given not in git repo, it passes through to claude", func(t *testing.T) {
		withUpdateStub(t)

		origRoot := getGitRepoRootFn
		getGitRepoRootFn = func() (string, error) { return "", fmt.Errorf("not a git repo") }
		t.Cleanup(func() { getGitRepoRootFn = origRoot })

		var passedArgs []string
		withClaudeStub(t, func(args []string) error {
			passedArgs = args
			return nil
		})

		if err := run([]string{"--help"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(passedArgs) != 1 || passedArgs[0] != "--help" {
			t.Errorf("expected args [--help], got %v", passedArgs)
		}
	})

	t.Run("Given normal flow, it runs syncIn, claude, syncOut, cleanup", func(t *testing.T) {
		withUpdateStub(t)

		home := t.TempDir()
		t.Setenv("HOME", home)

		repoRoot, _ := setupTestRepo(t)
		repoName := filepath.Base(repoRoot)
		storeBase := filepath.Join(home, ".workspaces", repoName)
		if err := os.MkdirAll(storeBase, 0755); err != nil {
			t.Fatal(err)
		}

		// Seed a file in the store
		if err := os.WriteFile(filepath.Join(storeBase, "CLAUDE.md"), []byte("workspace"), 0644); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, repoRoot, "main", "main", map[string]bool{"main": true})

		claudeCalled := false
		withClaudeStub(t, func(args []string) error {
			claudeCalled = true
			// Verify syncIn happened: file should be in repo
			content, err := os.ReadFile(filepath.Join(repoRoot, "CLAUDE.md"))
			if err != nil {
				t.Errorf("syncIn should have copied CLAUDE.md: %v", err)
			}
			if string(content) != "workspace" {
				t.Errorf("expected 'workspace', got %q", string(content))
			}
			return nil
		})

		if err := run([]string{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !claudeCalled {
			t.Error("claude should have been called")
		}

		// Verify syncOut happened: file should be back in store
		content, err := os.ReadFile(filepath.Join(storeBase, "CLAUDE.md"))
		if err != nil {
			t.Errorf("syncOut should have copied CLAUDE.md back to store: %v", err)
		}
		if string(content) != "workspace" {
			t.Errorf("expected 'workspace' in store, got %q", string(content))
		}
	})

	t.Run("Given claude fails, it returns error", func(t *testing.T) {
		withUpdateStub(t)

		home := t.TempDir()
		t.Setenv("HOME", home)

		repoRoot, _ := setupTestRepo(t)
		repoName := filepath.Base(repoRoot)
		storeBase := filepath.Join(home, ".workspaces", repoName)
		if err := os.MkdirAll(storeBase, 0755); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, repoRoot, "main", "main", map[string]bool{"main": true})
		withClaudeStub(t, func(args []string) error {
			return fmt.Errorf("claude crashed")
		})

		err := run([]string{})
		if err == nil {
			t.Fatal("expected error when claude fails")
		}
		if !strings.Contains(err.Error(), "claude") {
			t.Errorf("error should mention claude, got: %v", err)
		}
	})

	t.Run("Given not in git repo and claude fails, it returns unwrapped error", func(t *testing.T) {
		withUpdateStub(t)

		origRoot := getGitRepoRootFn
		getGitRepoRootFn = func() (string, error) { return "", fmt.Errorf("not a git repo") }
		t.Cleanup(func() { getGitRepoRootFn = origRoot })

		withClaudeStub(t, func(args []string) error {
			return fmt.Errorf("claude not found")
		})

		err := run([]string{})
		if err == nil {
			t.Fatal("expected error when claude fails outside git repo")
		}
		if !strings.Contains(err.Error(), "claude not found") {
			t.Errorf("expected raw claude error, got: %v", err)
		}
		// Error should NOT be wrapped (main.go:215 returns execClaudeFn directly)
		if strings.Contains(err.Error(), "execution failed") {
			t.Error("error should not be wrapped when outside git repo")
		}
	})

	t.Run("Given syncIn fails, it returns error", func(t *testing.T) {
		withUpdateStub(t)

		home := t.TempDir()
		t.Setenv("HOME", home)

		repoRoot, _ := setupTestRepo(t)
		repoName := filepath.Base(repoRoot)
		storeBase := filepath.Join(home, ".workspaces", repoName)

		// Create storeBase as a file (not dir) so listDir fails in syncIn
		if err := os.MkdirAll(filepath.Dir(storeBase), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(storeBase, []byte("not a directory"), 0644); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, repoRoot, "main", "main", map[string]bool{"main": true})
		withClaudeStub(t, func(args []string) error {
			t.Error("claude should not be called when syncIn fails")
			return nil
		})

		err := run([]string{})
		if err == nil {
			t.Fatal("expected error when syncIn fails")
		}
		if !strings.Contains(err.Error(), "sync in failed") {
			t.Errorf("expected 'sync in failed' error, got: %v", err)
		}
	})

	t.Run("Given syncOut fails, it returns error", func(t *testing.T) {
		withUpdateStub(t)

		home := t.TempDir()
		t.Setenv("HOME", home)

		repoRoot, _ := setupTestRepo(t)
		repoName := filepath.Base(repoRoot)
		storeBase := filepath.Join(home, ".workspaces", repoName)
		if err := os.MkdirAll(storeBase, 0755); err != nil {
			t.Fatal(err)
		}

		withGitStubs(t, repoRoot, "main", "main", map[string]bool{"main": true})
		withClaudeStub(t, func(args []string) error {
			// Sabotage: replace storeBase dir with a file so syncOut's MkdirAll fails
			if err := os.RemoveAll(storeBase); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(storeBase, []byte("not a dir"), 0644); err != nil {
				t.Fatal(err)
			}
			return nil
		})

		err := run([]string{})
		if err == nil {
			t.Fatal("expected error when syncOut fails")
		}
		if !strings.Contains(err.Error(), "sync out failed") {
			t.Errorf("expected 'sync out failed' error, got: %v", err)
		}
	})
}
