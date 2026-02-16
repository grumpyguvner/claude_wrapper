package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
