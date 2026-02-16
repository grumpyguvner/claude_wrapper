package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

const (
	excludeFile      = ".git/info/exclude"
	deletionMarker   = ".deleted_at"
	branchesDir      = "branches"
	deletionGraceDays = 7
)

type Config struct {
	RepoRoot       string
	CurrentBranch  string
	DefaultBranch  string
	StoreBase      string
	StoreLocation  string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		log.Fatalf("error: %v", err)
	}
}

var versionTagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

type updater struct {
	version     string
	apiURL      string
	downloadURL func(tag string) string
	client      *http.Client
	dlClient    *http.Client
	selfPath    func() (string, error)
	restart     func(self string) error
}

func defaultUpdater() *updater {
	return &updater{
		version: version,
		apiURL:  "https://api.github.com/repos/grumpyguvner/claude_wrapper/releases/latest",
		downloadURL: func(tag string) string {
			return fmt.Sprintf("https://github.com/grumpyguvner/claude_wrapper/releases/download/%s/claude-wrapper", tag)
		},
		client:   &http.Client{Timeout: 5 * time.Second},
		dlClient: &http.Client{Timeout: 60 * time.Second},
		selfPath: func() (string, error) {
			self, err := os.Executable()
			if err != nil {
				return "", err
			}
			return filepath.EvalSymlinks(self)
		},
		restart: func(self string) error {
			return syscall.Exec(self, os.Args, os.Environ())
		},
	}
}

// checkLatest fetches the latest release tag. Returns "" if no update is needed.
func (u *updater) checkLatest() (string, error) {
	if u.version == "dev" {
		return "", nil
	}

	resp, err := u.client.Get(u.apiURL)
	if err != nil {
		return "", fmt.Errorf("update check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update check returned status %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %w", err)
	}

	if release.TagName == "" || release.TagName == u.version {
		return "", nil
	}

	if !versionTagPattern.MatchString(release.TagName) {
		return "", fmt.Errorf("unexpected release tag format: %s", release.TagName)
	}

	return release.TagName, nil
}

// downloadAndReplace downloads the new binary and replaces the current executable.
func (u *updater) downloadAndReplace(tag string) error {
	self, err := u.selfPath()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	dlResp, err := u.dlClient.Get(u.downloadURL(tag))
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned status %d", dlResp.StatusCode)
	}

	// Create temp file in same directory as binary to ensure atomic rename
	tmpFile, err := os.CreateTemp(filepath.Dir(self), ".claude-wrapper-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	if _, err := io.Copy(tmpFile, dlResp.Body); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write update: %w", err)
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to chmod update: %w", err)
	}

	if err := os.Rename(tmpPath, self); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// apply performs the full update check, download, replace, and restart cycle.
func (u *updater) apply() {
	if os.Getenv("CLAUDE_WRAPPER_UPDATED") == "1" {
		return
	}

	tag, err := u.checkLatest()
	if err != nil {
		log.Printf("warning: %v", err)
		return
	}
	if tag == "" {
		return
	}

	log.Printf("updating claude-wrapper from %s to %s...", u.version, tag)

	if err := u.downloadAndReplace(tag); err != nil {
		log.Printf("warning: %v", err)
		return
	}

	self, err := u.selfPath()
	if err != nil {
		log.Printf("warning: updated to %s but failed to resolve executable path: %v", tag, err)
		return
	}

	os.Setenv("CLAUDE_WRAPPER_UPDATED", "1")
	log.Printf("updated to %s, restarting...", tag)
	if err := u.restart(self); err != nil {
		log.Printf("warning: updated but failed to restart: %v", err)
	}
}

func checkForUpdate() {
	defaultUpdater().apply()
}

func run(args []string) error {
	checkForUpdate()

	cfg, err := loadConfig()
	if err != nil {
		// Not in a git repo, just pass through to claude
		return execClaude(args)
	}

	// Sync in: storage -> working directory
	if err := syncIn(cfg); err != nil {
		return fmt.Errorf("sync in failed: %w", err)
	}

	// Execute claude
	if err := execClaude(args); err != nil {
		return fmt.Errorf("claude execution failed: %w", err)
	}

	// Sync out: working directory -> storage
	if err := syncOut(cfg); err != nil {
		return fmt.Errorf("sync out failed: %w", err)
	}

	// Cleanup old branches
	if err := cleanupDeletedBranches(cfg); err != nil {
		// Log but don't fail on cleanup errors
		log.Printf("warning: cleanup failed: %v", err)
	}

	return nil
}

func loadConfig() (*Config, error) {
	repoRoot, err := getGitRepoRoot()
	if err != nil {
		return nil, err
	}

	currentBranch, err := getCurrentBranch()
	if err != nil {
		return nil, err
	}

	defaultBranch := getDefaultBranch()
	repoName := filepath.Base(repoRoot)
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	storeBase := filepath.Join(homeDir, ".workspaces", repoName)
	
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
	}, nil
}

func getGitRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getCurrentBranch() (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("not on a branch")
	}
	return branch, nil
}

func getDefaultBranch() string {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "main"
	}
	ref := strings.TrimSpace(string(output))
	return strings.TrimPrefix(ref, "refs/remotes/origin/")
}

func getAllBranches() (map[string]bool, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	
	branches := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		branch := strings.TrimSpace(scanner.Text())
		if branch != "" {
			branches[branch] = true
		}
	}
	return branches, scanner.Err()
}

func encodeBranchName(branch string) string {
	return url.PathEscape(branch)
}

func decodeBranchName(encoded string) (string, error) {
	return url.PathUnescape(encoded)
}

func syncIn(cfg *Config) error {
	// Initialize branch storage if needed
	if err := initializeBranchStorage(cfg); err != nil {
		return err
	}

	// Get items from storage
	items, err := listDir(cfg.StoreLocation)
	if err != nil {
		return err
	}

	// Filter out special items
	items = filterItems(items)

	// Copy from storage to working directory
	for _, item := range items {
		src := filepath.Join(cfg.StoreLocation, item)
		dst := filepath.Join(cfg.RepoRoot, item)
		if err := copyPath(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s: %w", item, err)
		}

		// Add to git exclude
		if err := addToExclude(cfg.RepoRoot, item); err != nil {
			return fmt.Errorf("failed to update exclude for %s: %w", item, err)
		}
	}

	return nil
}

func initializeBranchStorage(cfg *Config) error {
	// Default branch uses storeBase directly, no initialization needed
	if cfg.CurrentBranch == cfg.DefaultBranch {
		return nil
	}

	// Check if storage already exists
	_, err := os.Stat(cfg.StoreLocation)
	if err == nil {
		return nil // Already exists
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check branch storage %s: %w", cfg.StoreLocation, err)
	}

	// Create new branch storage directory
	if err := os.MkdirAll(cfg.StoreLocation, 0755); err != nil {
		return err
	}

	// Copy from default branch if it exists
	if _, err := os.Stat(cfg.StoreBase); err == nil {
		items, err := listDir(cfg.StoreBase)
		if err != nil {
			return err
		}

		for _, item := range items {
			// Skip branches directory and markers
			if item == branchesDir || item == deletionMarker {
				continue
			}

			src := filepath.Join(cfg.StoreBase, item)
			dst := filepath.Join(cfg.StoreLocation, item)
			if err := copyPath(src, dst); err != nil {
				return fmt.Errorf("failed to copy %s from default branch: %w", item, err)
			}
		}
	}

	return nil
}

func syncOut(cfg *Config) error {
	// Get items from exclude file
	excludeItems, err := readExcludeFile(cfg.RepoRoot)
	if err != nil {
		return err
	}

	// Create storage directory if needed
	if err := os.MkdirAll(cfg.StoreLocation, 0755); err != nil {
		return err
	}

	// Copy excluded items to storage
	for _, item := range excludeItems {
		src := filepath.Join(cfg.RepoRoot, item)
		if _, err := os.Stat(src); err != nil {
			continue // Item doesn't exist
		}

		dst := filepath.Join(cfg.StoreLocation, item)
		if err := copyPath(src, dst); err != nil {
			return fmt.Errorf("failed to copy %s to storage: %w", item, err)
		}
	}

	// Remove items from storage that aren't in exclude file
	storageItems, err := listDir(cfg.StoreLocation)
	if err != nil {
		return err
	}

	excludeMap := make(map[string]bool)
	for _, item := range excludeItems {
		excludeMap[item] = true
	}

	for _, item := range storageItems {
		// Skip special items
		if item == deletionMarker || item == branchesDir {
			continue
		}

		if !excludeMap[item] {
			path := filepath.Join(cfg.StoreLocation, item)
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("failed to remove %s from storage: %w", item, err)
			}
		}
	}

	return nil
}

func cleanupDeletedBranches(cfg *Config) error {
	branchesPath := filepath.Join(cfg.StoreBase, branchesDir)
	
	// Check if branches directory exists
	if _, err := os.Stat(branchesPath); os.IsNotExist(err) {
		return nil
	}

	// Get all current git branches
	gitBranches, err := getAllBranches()
	if err != nil {
		return err
	}

	// List all stored branch directories
	entries, err := os.ReadDir(branchesPath)
	if err != nil {
		return err
	}

	now := time.Now()
	gracePeriod := deletionGraceDays * 24 * time.Hour

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		encodedName := entry.Name()
		branchPath := filepath.Join(branchesPath, encodedName)
		markerPath := filepath.Join(branchPath, deletionMarker)

		branchName, err := decodeBranchName(encodedName)
		if err != nil {
			log.Printf("warning: failed to decode branch name %s: %v", encodedName, err)
			continue
		}

		// Skip current branch
		if branchName == cfg.CurrentBranch {
			continue
		}

		// Check if branch exists in git
		if gitBranches[branchName] {
			// Branch exists - remove marker if present
			os.Remove(markerPath)
			continue
		}

		// Branch doesn't exist in git
		markerExists := false
		if data, err := os.ReadFile(markerPath); err == nil {
			markerExists = true
			
			// Check age of marker
			timestamp, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
			if err == nil {
				deletedAt := time.Unix(timestamp, 0)
				if now.Sub(deletedAt) > gracePeriod {
					// Delete the branch directory
					if err := os.RemoveAll(branchPath); err != nil {
						log.Printf("warning: failed to delete old branch %s: %v", branchName, err)
					}
				}
			}
		}

		// Create marker if it doesn't exist
		if !markerExists {
			timestamp := strconv.FormatInt(now.Unix(), 10)
			if err := os.WriteFile(markerPath, []byte(timestamp), 0644); err != nil {
				log.Printf("warning: failed to create deletion marker for %s: %v", branchName, err)
			}
		}
	}

	return nil
}

func listDir(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var items []string
	for _, entry := range entries {
		items = append(items, entry.Name())
	}
	return items, nil
}

func filterItems(items []string) []string {
	var filtered []string
	for _, item := range items {
		if item == deletionMarker || item == branchesDir {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func readExcludeFile(repoRoot string) ([]string, error) {
	excludePath := filepath.Join(repoRoot, excludeFile)
	
	file, err := os.Open(excludePath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var items []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Skip patterns with wildcards
		if strings.ContainsAny(line, "*?[]") {
			continue
		}

		// Remove trailing slash
		line = strings.TrimSuffix(line, "/")

		// Check if item exists
		itemPath := filepath.Join(repoRoot, line)
		if _, err := os.Stat(itemPath); err == nil {
			items = append(items, line)
		}
	}

	return items, scanner.Err()
}

func addToExclude(repoRoot, item string) error {
	excludePath := filepath.Join(repoRoot, excludeFile)
	
	// Ensure .git/info directory exists
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		return err
	}

	// Check if item already exists in exclude file
	if file, err := os.Open(excludePath); err == nil {
		scanner := bufio.NewScanner(file)
		found := false
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == item {
				found = true
				break
			}
		}
		file.Close()
		if found {
			return nil
		}
	}

	// Append to exclude file
	file, err := os.OpenFile(excludePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = fmt.Fprintf(file, "%s\n", item)
	return err
}

func copyPath(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}

	if srcInfo.Mode()&os.ModeSymlink != 0 {
		log.Printf("warning: skipping symlink: %s", src)
		return nil
	}

	if srcInfo.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy permissions
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, srcInfo.Mode())
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.Type()&os.ModeSymlink != 0 {
			log.Printf("warning: skipping symlink: %s", srcPath)
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func execClaude(args []string) error {
	cmd := exec.Command("claude", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
