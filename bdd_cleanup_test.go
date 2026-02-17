package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// --- Scenario 5: Deleted Branch Cleanup ---

func TestScenario_UserDeletesFeatureBranch(t *testing.T) {
	t.Run("Given feature branch experiment was deleted from git", func(t *testing.T) {
		t.Run("And its storage still exists", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, storeBase := givenConfig(t, repoRoot, configOpts{})
			branchesPath := filepath.Join(storeBase, branchesDir)

			writeFile(t, filepath.Join(branchesPath, "experiment", "CLAUDE.md"), "experiment config")

			withBranches(t, map[string]bool{"main": true})

			t.Run("When the wrapper runs cleanup", func(t *testing.T) {
				if err := cleanupDeletedBranches(cfg); err != nil {
					t.Fatalf("cleanup failed: %v", err)
				}

				t.Run("Then a deletion marker is created", func(t *testing.T) {
					markerPath := filepath.Join(branchesPath, "experiment", deletionMarker)
					assertExists(t, markerPath)

					content := readFileContent(t, markerPath)
					ts, err := strconv.ParseInt(strings.TrimSpace(content), 10, 64)
					if err != nil {
						t.Fatalf("marker is not a valid timestamp: %v", err)
					}
					if time.Since(time.Unix(ts, 0)) > 5*time.Second {
						t.Error("marker timestamp is not recent")
					}
				})

				t.Run("Then the branch files are preserved during the grace period", func(t *testing.T) {
					assertExists(t, filepath.Join(branchesPath, "experiment", "CLAUDE.md"))
				})
			})
		})
	})
}

func TestScenario_DeletedBranchStorageExpiresAfterGracePeriod(t *testing.T) {
	t.Run("Given a branch was deleted more than 7 days ago", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, storeBase := givenConfig(t, repoRoot, configOpts{})
		branchesPath := filepath.Join(storeBase, branchesDir)

		writeFile(t, filepath.Join(branchesPath, "old-feature", "CLAUDE.md"), "old config")

		expiredTs := time.Now().Add(-8 * 24 * time.Hour).Unix()
		writeFile(t, filepath.Join(branchesPath, "old-feature", deletionMarker),
			fmt.Sprintf("%d", expiredTs))

		withBranches(t, map[string]bool{"main": true})

		t.Run("When the wrapper runs cleanup", func(t *testing.T) {
			if err := cleanupDeletedBranches(cfg); err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}

			t.Run("Then the entire branch storage directory is removed", func(t *testing.T) {
				assertNotExists(t, filepath.Join(branchesPath, "old-feature"))
			})
		})
	})
}

func TestScenario_UserRecreatesPreviouslyDeletedBranch(t *testing.T) {
	t.Run("Given a branch was deleted and has a deletion marker", func(t *testing.T) {
		t.Run("And the user has since re-created the branch in git", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, storeBase := givenConfig(t, repoRoot, configOpts{})
			branchesPath := filepath.Join(storeBase, branchesDir)

			writeFile(t, filepath.Join(branchesPath, "revived", "CLAUDE.md"), "revived config")
			writeFile(t, filepath.Join(branchesPath, "revived", deletionMarker), "12345")

			// Branch exists again in git
			withBranches(t, map[string]bool{"main": true, "revived": true})

			t.Run("When the wrapper runs cleanup", func(t *testing.T) {
				if err := cleanupDeletedBranches(cfg); err != nil {
					t.Fatalf("cleanup failed: %v", err)
				}

				t.Run("Then the deletion marker is removed", func(t *testing.T) {
					assertNotExists(t, filepath.Join(branchesPath, "revived", deletionMarker))
				})

				t.Run("Then the branch storage is preserved", func(t *testing.T) {
					assertFileContent(t, filepath.Join(branchesPath, "revived", "CLAUDE.md"), "revived config")
				})
			})
		})
	})
}

func TestScenario_CurrentBranchIsNeverCleanedUp(t *testing.T) {
	t.Run("Given the user is currently on branch my-feature", func(t *testing.T) {
		t.Run("And my-feature does not appear in git branch output", func(t *testing.T) {
			repoRoot := givenRepo(t)
			storeBase := t.TempDir()
			branchesPath := filepath.Join(storeBase, branchesDir)

			writeFile(t, filepath.Join(branchesPath, "my-feature", "CLAUDE.md"), "my feature config")

			// my-feature is NOT in git branches (edge case)
			withBranches(t, map[string]bool{"main": true})

			cfg := &Config{
				RepoRoot:      repoRoot,
				CurrentBranch: "my-feature",
				DefaultBranch: "main",
				StoreBase:     storeBase,
				StoreLocation: filepath.Join(branchesPath, "my-feature"),
			}

			t.Run("When the wrapper runs cleanup", func(t *testing.T) {
				if err := cleanupDeletedBranches(cfg); err != nil {
					t.Fatalf("cleanup failed: %v", err)
				}

				t.Run("Then my-feature storage is not touched", func(t *testing.T) {
					assertExists(t, filepath.Join(branchesPath, "my-feature", "CLAUDE.md"))
					assertNotExists(t, filepath.Join(branchesPath, "my-feature", deletionMarker))
				})
			})
		})
	})
}
