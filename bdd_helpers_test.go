package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// --- BDD helpers ---

type configOpts struct {
	currentBranch string
	defaultBranch string
}

// givenRepo sets up a fake repo root with .git/info/ and returns the path.
func givenRepo(t *testing.T) string {
	t.Helper()
	return setupRepoRoot(t)
}

// givenConfig builds a Config for testing with the given branch setup.
// Returns the Config and the store base path.
func givenConfig(t *testing.T, repoRoot string, opts configOpts) (*Config, string) {
	t.Helper()
	if opts.defaultBranch == "" {
		opts.defaultBranch = "main"
	}
	if opts.currentBranch == "" {
		opts.currentBranch = opts.defaultBranch
	}

	storeBase := t.TempDir()
	var storeLocation string
	if opts.currentBranch == opts.defaultBranch {
		storeLocation = storeBase
	} else {
		storeLocation = filepath.Join(storeBase, branchesDir, opts.currentBranch)
	}

	cfg := &Config{
		RepoRoot:      repoRoot,
		CurrentBranch: opts.currentBranch,
		DefaultBranch: opts.defaultBranch,
		StoreBase:     storeBase,
		StoreLocation: storeLocation,
	}
	return cfg, storeBase
}

// assertExcludeContains checks that the exclude file contains the given entry.
func assertExcludeContains(t *testing.T, repoRoot, entry string) {
	t.Helper()
	content := readFileContent(t, filepath.Join(repoRoot, excludeFile))
	if !strings.Contains(content, entry) {
		t.Errorf("expected exclude file to contain %q, got:\n%s", entry, content)
	}
}

// assertExcludeCount checks that an entry appears exactly n times in the exclude file.
func assertExcludeCount(t *testing.T, repoRoot, entry string, n int) {
	t.Helper()
	content := readFileContent(t, filepath.Join(repoRoot, excludeFile))
	got := strings.Count(content, entry)
	if got != n {
		t.Errorf("expected %q to appear %d time(s) in exclude file, got %d", entry, n, got)
	}
}

// withBranches sets up a mock getAllBranchesFunc and restores the original on cleanup.
func withBranches(t *testing.T, branches map[string]bool) {
	t.Helper()
	orig := getAllBranchesFunc
	getAllBranchesFunc = func() (map[string]bool, error) {
		return branches, nil
	}
	t.Cleanup(func() { getAllBranchesFunc = orig })
}
