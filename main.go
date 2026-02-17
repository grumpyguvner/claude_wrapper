package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

var (
	Version   = "dev"
	BuildTime = "unknown"
)

const (
	excludeFile       = ".git/info/exclude"
	deletionMarker    = ".deleted_at"
	branchesDir       = "branches"
	deletionGraceDays = 7
)

type Config struct {
	RepoRoot      string
	CurrentBranch string
	DefaultBranch string
	StoreBase     string
	StoreLocation string
}

// sanitizeBranchName percent-encodes characters that would create nested
// directories or are otherwise unsafe in filesystem paths.
func sanitizeBranchName(branch string) string {
	branch = strings.ReplaceAll(branch, "%", "%25")
	branch = strings.ReplaceAll(branch, "/", "%2F")
	return branch
}

// unsanitizeBranchName reverses sanitizeBranchName.
func unsanitizeBranchName(name string) string {
	name = strings.ReplaceAll(name, "%2F", "/")
	name = strings.ReplaceAll(name, "%25", "%")
	return name
}

func main() {
	exitCode, err := run(os.Args[1:])
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	os.Exit(exitCode)
}

func run(args []string) (int, error) {
	cfg, err := loadConfig()
	if err != nil {
		// Not in a git repo, just exec claude directly (replaces process)
		return 0, execClaude(args)
	}

	// Sync in: storage -> working directory
	if err := syncIn(cfg); err != nil {
		return 0, fmt.Errorf("sync in failed: %w", err)
	}

	// Execute claude and capture exit code
	claudeExit := runClaude(args)

	// Sync out: always run regardless of claude's exit code
	if err := syncOut(cfg); err != nil {
		return claudeExit, fmt.Errorf("sync out failed: %w", err)
	}

	// Cleanup old branches
	if err := cleanupDeletedBranches(cfg); err != nil {
		log.Printf("warning: cleanup failed: %v", err)
	}

	return claudeExit, nil
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
		storeLocation = filepath.Join(storeBase, branchesDir, sanitizeBranchName(currentBranch))
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

// getAllBranchesFunc is the function used to get git branches. Replaced in tests.
var getAllBranchesFunc = getAllBranches

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
	// Nothing to do on default branch
	if cfg.CurrentBranch == cfg.DefaultBranch {
		return nil
	}

	// Nothing to do if storage already exists
	if _, err := os.Stat(cfg.StoreLocation); err == nil {
		return nil
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
	gitBranches, err := getAllBranchesFunc()
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

		dirName := entry.Name()
		branchName := unsanitizeBranchName(dirName)
		branchPath := filepath.Join(branchesPath, dirName)
		markerPath := filepath.Join(branchPath, deletionMarker)

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
	if readFile, err := os.Open(excludePath); err == nil {
		scanner := bufio.NewScanner(readFile)
		found := false
		for scanner.Scan() {
			if strings.TrimSpace(scanner.Text()) == item {
				found = true
				break
			}
		}
		readFile.Close()
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to read exclude file: %w", err)
		}
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
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
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

// execClaude replaces the current process with claude (used for non-git pass-through).
func execClaude(args []string) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude not found: %w", err)
	}
	return syscall.Exec(claudePath, append([]string{"claude"}, args...), os.Environ())
}

// runClaude runs claude as a subprocess and returns its exit code.
func runClaude(args []string) int {
	cmd := exec.Command("claude", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		return 1
	}
	return 0
}
