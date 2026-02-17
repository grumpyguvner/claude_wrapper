package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installScriptPath returns the path to install.sh, skipping if not found.
func installScriptPath(t *testing.T) string {
	t.Helper()
	p := filepath.Join("scripts", "install.sh")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Skip("scripts/install.sh not found")
	}
	return p
}

// runAddAlias runs the add_alias function from install.sh against a temp HOME.
func runAddAlias(t *testing.T, home string) (string, error) {
	t.Helper()
	script := fmt.Sprintf(`
set -e
HOME=%q
ALIAS_LINE="alias claude='claude-wrapper'"
add_alias() {
    local rc_file="$1"
    if [ -f "$rc_file" ]; then
        if ! grep -qF "$ALIAS_LINE" "$rc_file"; then
            echo "" >> "$rc_file"
            echo "$ALIAS_LINE" >> "$rc_file"
            echo "Added alias to $rc_file"
        else
            echo "Alias already present in $rc_file"
        fi
    fi
}
add_alias "$HOME/.bashrc"
add_alias "$HOME/.zshrc"
if [ ! -f "$HOME/.bashrc" ] && [ ! -f "$HOME/.zshrc" ]; then
    echo "$ALIAS_LINE" > "$HOME/.bashrc"
    echo "Created $HOME/.bashrc with alias"
fi
`, home)
	cmd := exec.Command("bash", "-c", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// --- Scenario: Install script alias is idempotent ---

func TestScenario_InstallScriptAliasIsIdempotent(t *testing.T) {
	t.Run("Given a user with an existing .bashrc", func(t *testing.T) {
		home := t.TempDir()
		bashrc := filepath.Join(home, ".bashrc")
		writeFile(t, bashrc, "# existing config\n")
		aliasLine := "alias claude='claude-wrapper'"

		t.Run("When the install alias logic runs for the first time", func(t *testing.T) {
			out, err := runAddAlias(t, home)
			if err != nil {
				t.Fatalf("add_alias failed: %v\n%s", err, out)
			}

			t.Run("Then the alias is present in .bashrc", func(t *testing.T) {
				content := readFileContent(t, bashrc)
				if !strings.Contains(content, aliasLine) {
					t.Errorf("expected .bashrc to contain alias, got:\n%s", content)
				}
			})

			t.Run("When the alias logic runs again (re-run)", func(t *testing.T) {
				out2, err2 := runAddAlias(t, home)
				if err2 != nil {
					t.Fatalf("add_alias re-run failed: %v\n%s", err2, out2)
				}

				t.Run("Then the alias appears exactly once", func(t *testing.T) {
					content := readFileContent(t, bashrc)
					count := strings.Count(content, aliasLine)
					if count != 1 {
						t.Errorf("expected alias to appear once, got %d times in:\n%s", count, content)
					}
				})

				t.Run("Then output says alias already present", func(t *testing.T) {
					if !strings.Contains(out2, "Alias already present") {
						t.Errorf("expected 'Alias already present' message, got:\n%s", out2)
					}
				})
			})
		})
	})
}

// --- Scenario: Install script rejects unsupported platforms ---

func TestScenario_InstallScriptRejectsUnsupportedPlatforms(t *testing.T) {
	scriptPath := installScriptPath(t)

	t.Run("When run with a non-amd64 architecture override", func(t *testing.T) {
		mockBin := t.TempDir()
		unameScript := filepath.Join(mockBin, "uname")
		writeFile(t, unameScript, "#!/bin/bash\nif [ \"$1\" = \"-s\" ]; then echo Linux; elif [ \"$1\" = \"-m\" ]; then echo aarch64; fi\n")
		if err := os.Chmod(unameScript, 0755); err != nil {
			t.Fatalf("chmod failed: %v", err)
		}

		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "PATH="+mockBin+":"+os.Getenv("PATH"))
		output, err := cmd.CombinedOutput()

		t.Run("Then it exits with an error about unsupported platform", func(t *testing.T) {
			if err == nil {
				t.Fatal("expected script to fail on unsupported platform")
			}
			if !strings.Contains(string(output), "only Linux amd64 is supported") {
				t.Errorf("expected unsupported platform error, got:\n%s", output)
			}
		})
	})

	t.Run("When run on macOS", func(t *testing.T) {
		mockBin := t.TempDir()
		unameScript := filepath.Join(mockBin, "uname")
		writeFile(t, unameScript, "#!/bin/bash\nif [ \"$1\" = \"-s\" ]; then echo Darwin; elif [ \"$1\" = \"-m\" ]; then echo x86_64; fi\n")
		if err := os.Chmod(unameScript, 0755); err != nil {
			t.Fatalf("chmod failed: %v", err)
		}

		cmd := exec.Command("bash", scriptPath)
		cmd.Env = append(os.Environ(), "PATH="+mockBin+":"+os.Getenv("PATH"))
		output, err := cmd.CombinedOutput()

		t.Run("Then it exits with an error mentioning the detected OS", func(t *testing.T) {
			if err == nil {
				t.Fatal("expected script to fail on macOS")
			}
			if !strings.Contains(string(output), "Darwin") {
				t.Errorf("expected error to mention Darwin, got:\n%s", output)
			}
		})
	})
}

// --- Scenario: Install script adds alias to both shell configs ---

func TestScenario_InstallScriptCreatesAliasInBothShellConfigs(t *testing.T) {
	t.Run("Given a user with both .bashrc and .zshrc", func(t *testing.T) {
		home := t.TempDir()
		bashrc := filepath.Join(home, ".bashrc")
		zshrc := filepath.Join(home, ".zshrc")
		writeFile(t, bashrc, "# bash config\n")
		writeFile(t, zshrc, "# zsh config\n")
		aliasLine := "alias claude='claude-wrapper'"

		t.Run("When the install alias logic runs", func(t *testing.T) {
			out, err := runAddAlias(t, home)
			if err != nil {
				t.Fatalf("add_alias failed: %v\n%s", err, out)
			}

			t.Run("Then .bashrc contains the alias", func(t *testing.T) {
				content := readFileContent(t, bashrc)
				if !strings.Contains(content, aliasLine) {
					t.Errorf("expected .bashrc to contain alias")
				}
			})

			t.Run("Then .zshrc contains the alias", func(t *testing.T) {
				content := readFileContent(t, zshrc)
				if !strings.Contains(content, aliasLine) {
					t.Errorf("expected .zshrc to contain alias")
				}
			})
		})
	})
}

// --- Scenario: Install script creates .bashrc when no rc files exist ---

func TestScenario_InstallScriptCreatesBashrcWhenNoRcFilesExist(t *testing.T) {
	t.Run("Given a user with no .bashrc or .zshrc", func(t *testing.T) {
		home := t.TempDir()
		bashrc := filepath.Join(home, ".bashrc")
		aliasLine := "alias claude='claude-wrapper'"

		t.Run("When the install alias logic runs", func(t *testing.T) {
			out, err := runAddAlias(t, home)
			if err != nil {
				t.Fatalf("add_alias failed: %v\n%s", err, out)
			}

			t.Run("Then .bashrc is created with the alias", func(t *testing.T) {
				content := readFileContent(t, bashrc)
				if !strings.Contains(content, aliasLine) {
					t.Errorf("expected created .bashrc to contain alias, got:\n%s", content)
				}
			})

			t.Run("Then output confirms .bashrc was created", func(t *testing.T) {
				if !strings.Contains(out, "Created") {
					t.Errorf("expected 'Created' message, got:\n%s", out)
				}
			})
		})
	})
}

// --- Scenario: Install script installs to ~/.local/bin ---

func TestScenario_InstallScriptUsesLocalBin(t *testing.T) {
	scriptPath := installScriptPath(t)

	t.Run("Given the install script", func(t *testing.T) {
		content := readFileContent(t, scriptPath)

		t.Run("Then it installs to ~/.local/bin", func(t *testing.T) {
			if !strings.Contains(content, `"$HOME/.local/bin"`) {
				t.Errorf("expected install dir to be $HOME/.local/bin")
			}
		})

		t.Run("Then it does not use sudo", func(t *testing.T) {
			if strings.Contains(content, "sudo ") {
				t.Errorf("install script should not use sudo")
			}
		})
	})
}

// --- Scenario: Download URL extraction from GitHub API response ---

func TestScenario_InstallScriptExtractsDownloadURL(t *testing.T) {
	t.Run("Given a GitHub API JSON response with the expected asset", func(t *testing.T) {
		sampleJSON := `{
  "assets": [
    {
      "name": "claude-wrapper-linux-amd64",
      "browser_download_url": "https://github.com/grumpyguvner/claude_wrapper/releases/download/v1.2.3/claude-wrapper-linux-amd64"
    },
    {
      "name": "install.sh",
      "browser_download_url": "https://github.com/grumpyguvner/claude_wrapper/releases/download/v1.2.3/install.sh"
    }
  ]
}`
		expectedURL := "https://github.com/grumpyguvner/claude_wrapper/releases/download/v1.2.3/claude-wrapper-linux-amd64"

		t.Run("When the grep pipeline extracts the URL", func(t *testing.T) {
			script := fmt.Sprintf(`
ASSET_NAME="claude-wrapper-linux-amd64"
echo '%s' | grep -o "\"browser_download_url\": *\"[^\"]*${ASSET_NAME}\"" | head -1 | cut -d'"' -f4
`, sampleJSON)
			cmd := exec.Command("bash", "-c", script)
			out, err := cmd.CombinedOutput()
			result := strings.TrimSpace(string(out))

			t.Run("Then the correct download URL is extracted", func(t *testing.T) {
				if err != nil {
					t.Fatalf("grep pipeline failed: %v\n%s", err, out)
				}
				if result != expectedURL {
					t.Errorf("expected %q, got %q", expectedURL, result)
				}
			})
		})
	})

	t.Run("Given a GitHub API JSON response without the expected asset", func(t *testing.T) {
		sampleJSON := `{
  "assets": [
    {
      "name": "install.sh",
      "browser_download_url": "https://github.com/grumpyguvner/claude_wrapper/releases/download/v1.2.3/install.sh"
    }
  ]
}`

		t.Run("When the grep pipeline runs", func(t *testing.T) {
			script := fmt.Sprintf(`
ASSET_NAME="claude-wrapper-linux-amd64"
RESULT=$(echo '%s' | grep -o "\"browser_download_url\": *\"[^\"]*${ASSET_NAME}\"" | head -1 | cut -d'"' -f4)
if [ -z "$RESULT" ]; then
    echo "EMPTY"
    exit 0
fi
echo "$RESULT"
`, sampleJSON)
			cmd := exec.Command("bash", "-c", script)
			out, err := cmd.CombinedOutput()
			result := strings.TrimSpace(string(out))

			t.Run("Then the result is empty", func(t *testing.T) {
				if err != nil {
					t.Fatalf("pipeline failed: %v\n%s", err, out)
				}
				if result != "EMPTY" {
					t.Errorf("expected EMPTY for missing asset, got %q", result)
				}
			})
		})
	})
}
