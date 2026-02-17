package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Scenario: Sync In Overwrites Stale Working Directory Files ---

func TestScenario_SyncInOverwritesStaleWorkingDirectoryFiles(t *testing.T) {
	t.Run("Given the working directory has files left over from a previous branch", func(t *testing.T) {
		t.Run("And storage has different content for the same file", func(t *testing.T) {
			repoRoot := givenRepo(t)
			cfg, _ := givenConfig(t, repoRoot, configOpts{})

			// Stale file in working directory from previous branch
			writeFile(t, filepath.Join(repoRoot, "CLAUDE.md"), "old branch content")
			// Storage has the correct content for current branch
			writeFile(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "current branch content")

			t.Run("When the wrapper syncs in", func(t *testing.T) {
				if err := syncIn(cfg); err != nil {
					t.Fatalf("syncIn failed: %v", err)
				}

				t.Run("Then the working directory file is overwritten with storage content", func(t *testing.T) {
					assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "current branch content")
				})
			})
		})
	})
}

// --- Scenario: Sync Out Creates Storage When It Does Not Exist ---

func TestScenario_SyncOutCreatesStorageDirectoryOnFirstRun(t *testing.T) {
	t.Run("Given the storage directory does not exist yet", func(t *testing.T) {
		repoRoot := givenRepo(t)
		storeBase := filepath.Join(t.TempDir(), "new-repo-store")
		storeLocation := filepath.Join(storeBase, branchesDir, "feature")

		cfg := &Config{
			RepoRoot:      repoRoot,
			CurrentBranch: "feature",
			DefaultBranch: "main",
			StoreBase:     storeBase,
			StoreLocation: storeLocation,
		}

		// User has a file in working directory listed in exclude
		writeFile(t, filepath.Join(repoRoot, "CLAUDE.md"), "new config")
		writeFile(t, filepath.Join(repoRoot, excludeFile), "CLAUDE.md\n")

		t.Run("When the wrapper syncs out", func(t *testing.T) {
			if err := syncOut(cfg); err != nil {
				t.Fatalf("syncOut failed: %v", err)
			}

			t.Run("Then the storage directory is created", func(t *testing.T) {
				assertExists(t, storeLocation)
			})

			t.Run("Then the file is persisted to the new storage", func(t *testing.T) {
				assertFileContent(t, filepath.Join(storeLocation, "CLAUDE.md"), "new config")
			})
		})
	})
}

// --- Scenario: Sync Out Skips Exclude Entries That Do Not Exist On Disk ---

func TestScenario_SyncOutSkipsNonexistentExcludeEntries(t *testing.T) {
	t.Run("Given the exclude file references a file that no longer exists on disk", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		// Exclude lists two files but only one exists
		writeFile(t, filepath.Join(repoRoot, "exists.md"), "I exist")
		writeFile(t, filepath.Join(repoRoot, excludeFile), "exists.md\ndeleted-file.md\n")

		t.Run("When the wrapper syncs out", func(t *testing.T) {
			if err := syncOut(cfg); err != nil {
				t.Fatalf("syncOut failed: %v", err)
			}

			t.Run("Then the existing file is saved to storage", func(t *testing.T) {
				assertFileContent(t, filepath.Join(cfg.StoreLocation, "exists.md"), "I exist")
			})

			t.Run("Then the nonexistent file is silently skipped", func(t *testing.T) {
				assertNotExists(t, filepath.Join(cfg.StoreLocation, "deleted-file.md"))
			})
		})
	})
}

// --- Scenario: New Personal File Created During Session Gets Persisted ---

func TestScenario_NewPersonalFileCreatedDuringSessionGetsPersisted(t *testing.T) {
	t.Run("Given the user starts a session with existing personal files", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		writeFile(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "existing config")

		t.Run("When the wrapper syncs in", func(t *testing.T) {
			if err := syncIn(cfg); err != nil {
				t.Fatalf("syncIn failed: %v", err)
			}

			t.Run("And the user creates a new personal file during the session", func(t *testing.T) {
				writeFile(t, filepath.Join(repoRoot, "new-notes.md"), "brand new notes")
				// User adds it to exclude (as the wrapper would on next sync-in)
				if err := addToExclude(repoRoot, "new-notes.md"); err != nil {
					t.Fatalf("addToExclude failed: %v", err)
				}

				t.Run("When the wrapper syncs out", func(t *testing.T) {
					if err := syncOut(cfg); err != nil {
						t.Fatalf("syncOut failed: %v", err)
					}

					t.Run("Then both the existing and new files are persisted to storage", func(t *testing.T) {
						assertFileContent(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "existing config")
						assertFileContent(t, filepath.Join(cfg.StoreLocation, "new-notes.md"), "brand new notes")
					})
				})
			})
		})
	})
}

// --- Scenario: Cleanup Handles Multiple Branches In Different States ---

func TestScenario_CleanupHandlesMultipleBranchesInDifferentStates(t *testing.T) {
	t.Run("Given multiple branch storage directories in different states", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, storeBase := givenConfig(t, repoRoot, configOpts{})
		branchesPath := filepath.Join(storeBase, branchesDir)

		// Branch still in git — should be untouched
		writeFile(t, filepath.Join(branchesPath, "active", "CLAUDE.md"), "active config")

		// Branch deleted, marker within grace period — should be kept
		writeFile(t, filepath.Join(branchesPath, "recent-delete", "CLAUDE.md"), "recent config")
		recentTs := time.Now().Add(-2 * 24 * time.Hour).Unix()
		writeFile(t, filepath.Join(branchesPath, "recent-delete", deletionMarker),
			fmt.Sprintf("%d", recentTs))

		// Branch deleted, marker expired — should be removed
		writeFile(t, filepath.Join(branchesPath, "old-delete", "CLAUDE.md"), "old config")
		expiredTs := time.Now().Add(-10 * 24 * time.Hour).Unix()
		writeFile(t, filepath.Join(branchesPath, "old-delete", deletionMarker),
			fmt.Sprintf("%d", expiredTs))

		// Branch deleted, no marker yet — should get a new marker
		writeFile(t, filepath.Join(branchesPath, "just-deleted", "CLAUDE.md"), "just deleted config")

		withBranches(t, map[string]bool{"main": true, "active": true})

		t.Run("When the wrapper runs cleanup", func(t *testing.T) {
			if err := cleanupDeletedBranches(cfg); err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}

			t.Run("Then the active branch is untouched", func(t *testing.T) {
				assertFileContent(t, filepath.Join(branchesPath, "active", "CLAUDE.md"), "active config")
				assertNotExists(t, filepath.Join(branchesPath, "active", deletionMarker))
			})

			t.Run("Then the recently deleted branch is kept with its marker", func(t *testing.T) {
				assertExists(t, filepath.Join(branchesPath, "recent-delete", "CLAUDE.md"))
				assertExists(t, filepath.Join(branchesPath, "recent-delete", deletionMarker))
			})

			t.Run("Then the expired branch is completely removed", func(t *testing.T) {
				assertNotExists(t, filepath.Join(branchesPath, "old-delete"))
			})

			t.Run("Then the newly deleted branch gets a fresh marker", func(t *testing.T) {
				assertExists(t, filepath.Join(branchesPath, "just-deleted", "CLAUDE.md"))
				assertExists(t, filepath.Join(branchesPath, "just-deleted", deletionMarker))
			})
		})
	})
}

// --- Scenario: Cleanup Ignores Non-Directory Entries in branches/ ---

func TestScenario_CleanupIgnoresStrayFilesInBranchesDirectory(t *testing.T) {
	t.Run("Given the branches directory contains a stray file alongside branch dirs", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, storeBase := givenConfig(t, repoRoot, configOpts{})
		branchesPath := filepath.Join(storeBase, branchesDir)

		// A legitimate branch directory
		writeFile(t, filepath.Join(branchesPath, "feature", "CLAUDE.md"), "feature config")
		// A stray file (not a directory) in branches/
		writeFile(t, filepath.Join(branchesPath, "stray-file.txt"), "should be ignored")

		withBranches(t, map[string]bool{"main": true, "feature": true})

		t.Run("When the wrapper runs cleanup", func(t *testing.T) {
			if err := cleanupDeletedBranches(cfg); err != nil {
				t.Fatalf("cleanup failed: %v", err)
			}

			t.Run("Then the stray file is left alone", func(t *testing.T) {
				assertFileContent(t, filepath.Join(branchesPath, "stray-file.txt"), "should be ignored")
			})

			t.Run("Then the branch directory is not affected", func(t *testing.T) {
				assertFileContent(t, filepath.Join(branchesPath, "feature", "CLAUDE.md"), "feature config")
			})
		})
	})
}

// --- Scenario: File Permissions Preserved Through Sync Roundtrip ---

func TestScenario_FilePermissionsPreservedThroughSyncRoundtrip(t *testing.T) {
	t.Run("Given a personal file with executable permissions in storage", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		scriptPath := filepath.Join(cfg.StoreLocation, "run.sh")
		writeFile(t, scriptPath, "#!/bin/bash\necho hello")
		if err := os.Chmod(scriptPath, 0755); err != nil {
			t.Fatalf("chmod failed: %v", err)
		}

		t.Run("When the wrapper syncs in and then syncs out", func(t *testing.T) {
			if err := syncIn(cfg); err != nil {
				t.Fatalf("syncIn failed: %v", err)
			}

			// Verify the file landed in working directory with correct permissions
			workingFile := filepath.Join(repoRoot, "run.sh")
			info, err := os.Stat(workingFile)
			if err != nil {
				t.Fatalf("stat failed: %v", err)
			}

			if info.Mode().Perm() != 0755 {
				t.Errorf("expected permissions 0755, got %o", info.Mode().Perm())
			}

			// Now sync out
			if err := syncOut(cfg); err != nil {
				t.Fatalf("syncOut failed: %v", err)
			}

			t.Run("Then the file permissions are preserved in storage", func(t *testing.T) {
				storageInfo, err := os.Stat(filepath.Join(cfg.StoreLocation, "run.sh"))
				if err != nil {
					t.Fatalf("stat failed: %v", err)
				}
				if storageInfo.Mode().Perm() != 0755 {
					t.Errorf("expected permissions 0755 in storage, got %o", storageInfo.Mode().Perm())
				}
			})
		})
	})
}

// --- Scenario: Branch Names With Slashes Create Correct Storage Paths ---

func TestScenario_BranchNamesWithSlashesWorkCorrectly(t *testing.T) {
	t.Run("Given the user is on a branch with slashes in the name", func(t *testing.T) {
		repoRoot := givenRepo(t)
		storeBase := t.TempDir()
		// Branch "feature/auth/oauth" should create nested path
		branchName := "feature/auth/oauth"
		storeLocation := filepath.Join(storeBase, branchesDir, branchName)

		cfg := &Config{
			RepoRoot:      repoRoot,
			CurrentBranch: branchName,
			DefaultBranch: "main",
			StoreBase:     storeBase,
			StoreLocation: storeLocation,
		}

		// Seed default branch with a file
		writeFile(t, filepath.Join(storeBase, "CLAUDE.md"), "default config")

		t.Run("When the wrapper syncs in", func(t *testing.T) {
			if err := syncIn(cfg); err != nil {
				t.Fatalf("syncIn failed: %v", err)
			}

			t.Run("Then the nested branch storage path is created correctly", func(t *testing.T) {
				assertExists(t, storeLocation)
				assertFileContent(t, filepath.Join(storeLocation, "CLAUDE.md"), "default config")
			})

			t.Run("Then the file appears in the working directory", func(t *testing.T) {
				assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "default config")
			})
		})
	})
}

// --- Scenario: Repeated Sync Cycles Are Idempotent ---

func TestScenario_RepeatedSyncCyclesAreIdempotent(t *testing.T) {
	t.Run("Given a session with personal files", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		writeFile(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "stable config")
		writeFile(t, filepath.Join(cfg.StoreLocation, "notes.md"), "stable notes")

		t.Run("When sync-in and sync-out are run multiple times", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				if err := syncIn(cfg); err != nil {
					t.Fatalf("syncIn iteration %d failed: %v", i, err)
				}
				if err := syncOut(cfg); err != nil {
					t.Fatalf("syncOut iteration %d failed: %v", i, err)
				}
			}

			t.Run("Then file content remains unchanged", func(t *testing.T) {
				assertFileContent(t, filepath.Join(cfg.StoreLocation, "CLAUDE.md"), "stable config")
				assertFileContent(t, filepath.Join(cfg.StoreLocation, "notes.md"), "stable notes")
				assertFileContent(t, filepath.Join(repoRoot, "CLAUDE.md"), "stable config")
				assertFileContent(t, filepath.Join(repoRoot, "notes.md"), "stable notes")
			})

			t.Run("Then exclude entries are not duplicated", func(t *testing.T) {
				assertExcludeCount(t, repoRoot, "CLAUDE.md", 1)
				assertExcludeCount(t, repoRoot, "notes.md", 1)
			})
		})
	})
}

// --- Scenario: Exclude File With Trailing Slash Directory Entry ---

func TestScenario_ExcludeFileTrailingSlashDirectoryIsHandledCorrectly(t *testing.T) {
	t.Run("Given the exclude file has a directory entry with a trailing slash", func(t *testing.T) {
		repoRoot := givenRepo(t)

		// Create the directory on disk
		if err := os.MkdirAll(filepath.Join(repoRoot, ".claude"), 0755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(repoRoot, ".claude", "config.json"), `{"key":"val"}`)

		// Exclude file has trailing slash
		writeFile(t, filepath.Join(repoRoot, excludeFile), ".claude/\n")

		t.Run("When the wrapper reads the exclude file", func(t *testing.T) {
			items, err := readExcludeFile(repoRoot)
			if err != nil {
				t.Fatalf("readExcludeFile failed: %v", err)
			}

			t.Run("Then the trailing slash is stripped and the directory is recognized", func(t *testing.T) {
				if len(items) != 1 {
					t.Fatalf("expected 1 item, got %d: %v", len(items), items)
				}
				if items[0] != ".claude" {
					t.Errorf("expected .claude, got %s", items[0])
				}
			})
		})
	})
}

// --- Scenario: Sync Out Persists Directory Entries ---

func TestScenario_SyncOutPersistsDirectoriesListedInExclude(t *testing.T) {
	t.Run("Given the user has a managed directory in the working tree", func(t *testing.T) {
		repoRoot := givenRepo(t)
		cfg, _ := givenConfig(t, repoRoot, configOpts{})

		// User has a .claude directory with nested content
		writeFile(t, filepath.Join(repoRoot, ".claude", "settings.json"), `{"editor":"vim"}`)
		writeFile(t, filepath.Join(repoRoot, ".claude", "prompts", "review.md"), "review prompt")
		writeFile(t, filepath.Join(repoRoot, excludeFile), ".claude\n")

		t.Run("When the wrapper syncs out", func(t *testing.T) {
			if err := syncOut(cfg); err != nil {
				t.Fatalf("syncOut failed: %v", err)
			}

			t.Run("Then the entire directory tree is persisted to storage", func(t *testing.T) {
				assertFileContent(t, filepath.Join(cfg.StoreLocation, ".claude", "settings.json"), `{"editor":"vim"}`)
				assertFileContent(t, filepath.Join(cfg.StoreLocation, ".claude", "prompts", "review.md"), "review prompt")
			})
		})
	})
}

// --- Scenario: First Run On Feature Branch With Empty Workspace ---

func TestScenario_FirstRunOnFeatureBranchWithNoStorageAnywhere(t *testing.T) {
	t.Run("Given the user has never used the wrapper before", func(t *testing.T) {
		t.Run("And they are on a feature branch", func(t *testing.T) {
			repoRoot := givenRepo(t)
			storeBase := filepath.Join(t.TempDir(), "brand-new-repo")
			storeLocation := filepath.Join(storeBase, branchesDir, "first-branch")

			cfg := &Config{
				RepoRoot:      repoRoot,
				CurrentBranch: "first-branch",
				DefaultBranch: "main",
				StoreBase:     storeBase,
				StoreLocation: storeLocation,
			}

			t.Run("When the wrapper syncs in", func(t *testing.T) {
				if err := syncIn(cfg); err != nil {
					t.Fatalf("syncIn failed: %v", err)
				}

				t.Run("Then branch storage is created empty", func(t *testing.T) {
					assertExists(t, storeLocation)
					items, _ := listDir(storeLocation)
					if len(items) != 0 {
						t.Errorf("expected empty storage, got %v", items)
					}
				})

				t.Run("Then no files appear in the working directory", func(t *testing.T) {
					entries, _ := os.ReadDir(repoRoot)
					for _, e := range entries {
						if e.Name() != ".git" {
							t.Errorf("unexpected file in working directory: %s", e.Name())
						}
					}
				})
			})
		})
	})
}
