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

// --- Scenario 1: Branch Storage Initialization ---

func TestScenario_UserSwitchesToNewFeatureBranch(t *testing.T) {
	t.Run("Given a new feature branch with no existing storage", func(t *testing.T) {
		t.Run("And the default branch has personal files", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, storeBase := givenConfig(t, repoRoot, configOpts{
				currentBranch: "feature/auth",
				defaultBranch: "main",
			})

			// Seed the default branch store with personal files
			writeFile(t, filepath.Join(storeBase, "CLAUDE.md"), "# My Config")
			writeFile(t, filepath.Join(storeBase, ".env.local"), "SECRET=abc")

			t.Run("When the wrapper syncs in", func(t *testing.T) {
				if err := syncIn(cfg); err != nil {
					t.Fatalf("syncIn failed: %v", err)
				}

				t.Run("Then branch storage is created seeded from default branch files", func(t *testing.T) {
					assertFileContent(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "# My Config")
					assertFileContent(t, filepath.Join(cfg.StoreLocation, ".env.local"), "SECRET=abc")
				})

				t.Run("Then the user sees their default branch files in the working directory", func(t *testing.T) {
					assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "# My Config")
					assertFileContent(t, filepath.Join(repoRoot, ".env.local"), "SECRET=abc")
				})
			})
		})
	})
}

func TestScenario_UserSwitchesToBranchWithExistingStorage(t *testing.T) {
	t.Run("Given a feature branch with existing storage", func(t *testing.T) {
		t.Run("And the branch storage has branch-specific modifications", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, storeBase := givenConfig(t, repoRoot, configOpts{
				currentBranch: "feature/custom",
				defaultBranch: "main",
			})

			// Seed default branch with original content
			writeFile(t, filepath.Join(storeBase, "CLAUDE.md"), "default config")

			// Pre-create branch storage with custom content
			os.MkdirAll(cfg.StoreLocation, 0755)
			writeFile(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "branch-specific config")

			t.Run("When the wrapper syncs in", func(t *testing.T) {
				if err := syncIn(cfg); err != nil {
					t.Fatalf("syncIn failed: %v", err)
				}

				t.Run("Then the existing branch-specific files are used not overwritten from default", func(t *testing.T) {
					assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "branch-specific config")
				})
			})
		})
	})
}

func TestScenario_UserIsOnDefaultBranch(t *testing.T) {
	t.Run("Given the user is on the default branch", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, storeBase := givenConfig(t, repoRoot, configOpts{
			currentBranch: "main",
			defaultBranch: "main",
		})

		writeFile(t, filepath.Join(storeBase, "CLAUDE.md"), "main branch config")

		t.Run("When the wrapper syncs in", func(t *testing.T) {
			if err := syncIn(cfg); err != nil {
				t.Fatalf("syncIn failed: %v", err)
			}

			t.Run("Then files come directly from the base store", func(t *testing.T) {
				// StoreLocation should equal StoreBase for default branch
				if cfg.StoreLocation != cfg.StoreBase {
					t.Errorf("expected StoreLocation to equal StoreBase for default branch")
				}
				assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "main branch config")
			})
		})
	})
}

// --- Scenario 2: File Synchronization (Sync In) ---

func TestScenario_UserStartsSessionWithPersonalFiles(t *testing.T) {
	t.Run("Given the user has CLAUDE.md and other personal files stored", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		writeFile(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "my config")
		writeFile(t, filepath.Join(cfg.StoreLocation, "notes.md"), "my notes")

		// Also add special items that should NOT be synced
		writeFile(t, filepath.Join(cfg.StoreLocation, deletionMarker), "12345")
		os.Mkdir(filepath.Join(cfg.StoreLocation, branchesDir), 0755)

		t.Run("When the wrapper syncs in before running claude", func(t *testing.T) {
			if err := syncIn(cfg); err != nil {
				t.Fatalf("syncIn failed: %v", err)
			}

			t.Run("Then all personal files appear in the working directory", func(t *testing.T) {
				assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "my config")
				assertFileContent(t, filepath.Join(repoRoot, "notes.md"), "my notes")
			})

			t.Run("Then all personal files are added to git exclude", func(t *testing.T) {
				assertExcludeContains(t, repoRoot, "CLAUDE.md")
				assertExcludeContains(t, repoRoot, "notes.md")
			})

			t.Run("Then special items are NOT synced to the working directory", func(t *testing.T) {
				assertNotExists(t, filepath.Join(repoRoot, deletionMarker))
				assertNotExists(t, filepath.Join(repoRoot, branchesDir))
			})
		})
	})
}

func TestScenario_UserStartsSessionWithNestedDirectories(t *testing.T) {
	t.Run("Given the user has a .claude/ directory with subdirectories in storage", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		writeFile(t, filepath.Join(cfg.StoreLocation, ".claude", "settings.json"), `{"theme":"dark"}`)
		writeFile(t, filepath.Join(cfg.StoreLocation, ".claude", "prompts", "review.md"), "review prompt")

		t.Run("When the wrapper syncs in", func(t *testing.T) {
			if err := syncIn(cfg); err != nil {
				t.Fatalf("syncIn failed: %v", err)
			}

			t.Run("Then the entire directory tree is copied preserving structure", func(t *testing.T) {
				assertFileContent(t, filepath.Join(repoRoot, ".claude", "settings.json"), `{"theme":"dark"}`)
				assertFileContent(t, filepath.Join(repoRoot, ".claude", "prompts", "review.md"), "review prompt")
			})
		})
	})
}

func TestScenario_UserStartsSessionWithEmptyStorage(t *testing.T) {
	t.Run("Given the storage directory is empty", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		t.Run("When the wrapper syncs in", func(t *testing.T) {
			err := syncIn(cfg)

			t.Run("Then no files are copied and no errors occur", func(t *testing.T) {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				// Working directory should only have .git
				entries, _ := os.ReadDir(repoRoot)
				for _, e := range entries {
					if e.Name() != ".git" {
						t.Errorf("unexpected file in working directory: %s", e.Name())
					}
				}
			})
		})
	})
}

// --- Scenario 3: File Synchronization (Sync Out) ---

func TestScenario_UserFinishesSessionAfterEditingFiles(t *testing.T) {
	t.Run("Given the user has modified CLAUDE.md during their session", func(t *testing.T) {
		t.Run("And CLAUDE.md is listed in git exclude", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, _ := givenConfig(t, repoRoot, configOpts{})

			// Simulate: file exists in working dir and is in exclude
			writeFile(t, filepath.Join(repoRoot, "CLAUDE.md"), "updated content from session")
			writeFile(t, filepath.Join(repoRoot, excludeFile), "CLAUDE.md\n")

			t.Run("When the wrapper syncs out after claude exits", func(t *testing.T) {
				if err := syncOut(cfg); err != nil {
					t.Fatalf("syncOut failed: %v", err)
				}

				t.Run("Then the updated CLAUDE.md is saved back to storage", func(t *testing.T) {
					assertFileContent(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "updated content from session")
				})
			})
		})
	})
}

func TestScenario_UserRemovesPersonalFileDuringSession(t *testing.T) {
	t.Run("Given a personal file existed in storage before the session", func(t *testing.T) {
		t.Run("And the user deleted that file during the session", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, _ := givenConfig(t, repoRoot, configOpts{})

			// Pre-populate storage with a file
			writeFile(t, filepath.Join(cfg.StoreLocation, "old-notes.md"), "old notes")

			// The file no longer exists in repo (user deleted it) and is not in exclude
			writeFile(t, filepath.Join(repoRoot, excludeFile), "")

			t.Run("When the wrapper syncs out", func(t *testing.T) {
				if err := syncOut(cfg); err != nil {
					t.Fatalf("syncOut failed: %v", err)
				}

				t.Run("Then the file is also removed from storage", func(t *testing.T) {
					assertNotExists(t, filepath.Join(cfg.StoreLocation, "old-notes.md"))
				})
			})
		})
	})
}

func TestScenario_UserSessionPreservesSpecialStorageItems(t *testing.T) {
	t.Run("Given branch storage contains special items", func(t *testing.T) {
		t.Run("And the exclude list does not reference these items", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, _ := givenConfig(t, repoRoot, configOpts{})

			// Storage has branches dir and deletion marker
			os.Mkdir(filepath.Join(cfg.StoreLocation, branchesDir), 0755)
			writeFile(t, filepath.Join(cfg.StoreLocation, deletionMarker), "12345")

			// Empty exclude — nothing managed by the user
			writeFile(t, filepath.Join(repoRoot, excludeFile), "")

			t.Run("When the wrapper syncs out", func(t *testing.T) {
				if err := syncOut(cfg); err != nil {
					t.Fatalf("syncOut failed: %v", err)
				}

				t.Run("Then branches dir and deletion marker are preserved in storage", func(t *testing.T) {
					assertExists(t, filepath.Join(cfg.StoreLocation, branchesDir))
					assertExists(t, filepath.Join(cfg.StoreLocation, deletionMarker))
				})
			})
		})
	})
}

// --- Scenario 4: Git Exclude Management ---

func TestScenario_UserPersonalFilesStayOutOfGit(t *testing.T) {
	t.Run("Given the user has personal files synced in", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		writeFile(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "config")
		writeFile(t, filepath.Join(cfg.StoreLocation, ".env.local"), "secrets")

		if err := syncIn(cfg); err != nil {
			t.Fatalf("syncIn failed: %v", err)
		}

		t.Run("When they check git exclude", func(t *testing.T) {
			t.Run("Then every personal file is listed in the exclude file", func(t *testing.T) {
				assertExcludeContains(t, repoRoot, "CLAUDE.md")
				assertExcludeContains(t, repoRoot, ".env.local")
			})

			t.Run("Then duplicate entries are not created on repeated syncs", func(t *testing.T) {
				// Run syncIn again
				if err := syncIn(cfg); err != nil {
					t.Fatalf("second syncIn failed: %v", err)
				}
				assertExcludeCount(t, repoRoot, "CLAUDE.md", 1)
				assertExcludeCount(t, repoRoot, ".env.local", 1)
			})
		})
	})
}

func TestScenario_ExcludeFileHandlesVariousFormats(t *testing.T) {
	t.Run("Given the exclude file contains comments blank lines and glob patterns", func(t *testing.T) {
		repoRoot := givenRepo(t)

		// Create files that exist on disk
		writeFile(t, filepath.Join(repoRoot, "CLAUDE.md"), "config")
		writeFile(t, filepath.Join(repoRoot, "notes.md"), "notes")

		excludeContent := "# This is a comment\n\nCLAUDE.md\n*.log\nnotes.md\n?temp\nnonexistent.txt\n"
		writeFile(t, filepath.Join(repoRoot, excludeFile), excludeContent)

		t.Run("When the wrapper reads the exclude file", func(t *testing.T) {
			items, err := readExcludeFile(repoRoot)
			if err != nil {
				t.Fatalf("readExcludeFile failed: %v", err)
			}

			t.Run("Then only plain file entries that exist on disk are processed", func(t *testing.T) {
				expected := map[string]bool{"CLAUDE.md": true, "notes.md": true}
				if len(items) != len(expected) {
					t.Fatalf("expected %d items, got %d: %v", len(expected), len(items), items)
				}
				for _, item := range items {
					if !expected[item] {
						t.Errorf("unexpected item: %s", item)
					}
				}
			})
		})
	})
}

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

// --- Scenario 6: Full Workflow Integration ---

func TestScenario_CompleteSessionLifecycleOnFeatureBranch(t *testing.T) {
	t.Run("Given the user is on feature branch add-auth", func(t *testing.T) {
		t.Run("And the default branch has CLAUDE.md in storage", func(t *testing.T) {
			t.Run("And this is the first time on add-auth", func(t *testing.T) {
				repoRoot := givenRepo(t)
				cfg, storeBase := givenConfig(t, repoRoot, configOpts{
					currentBranch: "add-auth",
					defaultBranch: "main",
				})

				writeFile(t, filepath.Join(storeBase, "CLAUDE.md"), "default CLAUDE.md")

				t.Run("When the full sync-in then edit then sync-out cycle runs", func(t *testing.T) {
					// Step 1: Sync in — initializes branch storage from default
					if err := syncIn(cfg); err != nil {
						t.Fatalf("syncIn failed: %v", err)
					}

					t.Run("Then branch storage is initialized from default", func(t *testing.T) {
						assertFileContent(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "default CLAUDE.md")
					})

					t.Run("Then files appear in working directory", func(t *testing.T) {
						assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "default CLAUDE.md")
					})

					// Step 2: Simulate user editing the file
					writeFile(t, filepath.Join(repoRoot, "CLAUDE.md"), "add-auth specific config")

					// Step 3: Sync out
					if err := syncOut(cfg); err != nil {
						t.Fatalf("syncOut failed: %v", err)
					}

					t.Run("Then edits are persisted back to branch-specific storage", func(t *testing.T) {
						assertFileContent(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "add-auth specific config")
					})

					t.Run("Then the default branch storage is NOT modified", func(t *testing.T) {
						assertFileContent(t, filepath.Join(storeBase, "CLAUDE.md"), "default CLAUDE.md")
					})
				})
			})
		})
	})
}

func TestScenario_MultipleBranchesMaintainIndependentFileStates(t *testing.T) {
	t.Run("Given two branches with different CLAUDE.md content", func(t *testing.T) {
		repoRoot := givenRepo(t)
		storeBase := t.TempDir()

		// Set up branch-specific stores
		featureAStore := filepath.Join(storeBase, branchesDir, "feature-a")
		featureBStore := filepath.Join(storeBase, branchesDir, "feature-b")

		writeFile(t, filepath.Join(featureAStore, "CLAUDE.md"), "Feature A config")
		writeFile(t, filepath.Join(featureBStore, "CLAUDE.md"), "Feature B config")

		cfgA := &Config{
			RepoRoot:      repoRoot,
			CurrentBranch: "feature-a",
			DefaultBranch: "main",
			StoreBase:     storeBase,
			StoreLocation: featureAStore,
		}

		cfgB := &Config{
			RepoRoot:      repoRoot,
			CurrentBranch: "feature-b",
			DefaultBranch: "main",
			StoreBase:     storeBase,
			StoreLocation: featureBStore,
		}

		t.Run("When syncing in for feature-a", func(t *testing.T) {
			if err := syncIn(cfgA); err != nil {
				t.Fatalf("syncIn for feature-a failed: %v", err)
			}

			t.Run("Then CLAUDE.md contains Feature A config", func(t *testing.T) {
				assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "Feature A config")
			})
		})

		t.Run("When syncing in for feature-b", func(t *testing.T) {
			if err := syncIn(cfgB); err != nil {
				t.Fatalf("syncIn for feature-b failed: %v", err)
			}

			t.Run("Then CLAUDE.md contains Feature B config", func(t *testing.T) {
				assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "Feature B config")
			})
		})
	})
}
