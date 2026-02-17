package main

import (
	"path/filepath"
	"testing"
)

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
