package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Git.TagPrefix != "v" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Git.TagPrefix, "v")
	}
	if cfg.Maven.PomPath != "pom.xml" {
		t.Errorf("PomPath = %q, want %q", cfg.Maven.PomPath, "pom.xml")
	}
}

func TestLoadOverride(t *testing.T) {
	dir := t.TempDir()
	content := `
git:
  tag_prefix: ""
  commit_message: "release: {version}"
maven:
  pom_path: "services/api/pom.xml"
`
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Git.TagPrefix != "" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Git.TagPrefix, "")
	}
	if cfg.Git.CommitMessage != "release: {version}" {
		t.Errorf("CommitMessage = %q", cfg.Git.CommitMessage)
	}
	if cfg.Maven.PomPath != "services/api/pom.xml" {
		t.Errorf("PomPath = %q", cfg.Maven.PomPath)
	}
}

func TestLoadReadError(t *testing.T) {
	dir := t.TempDir()
	// Create a directory named .releaser.yml so os.ReadFile fails (is a directory)
	if err := os.Mkdir(filepath.Join(dir, filename), 0755); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error when .releaser.yml is a directory")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte("{[invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestApplyEnv(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "mytoken")
	t.Setenv("CI_SERVER_URL", "https://gitlab.example.com")
	t.Setenv("CI_PROJECT_ID", "42")

	cfg := defaults()
	cfg.ApplyEnv()

	if cfg.GitLab.Token != "mytoken" {
		t.Errorf("Token = %q, want mytoken", cfg.GitLab.Token)
	}
	if cfg.GitLab.URL != "https://gitlab.example.com" {
		t.Errorf("URL = %q", cfg.GitLab.URL)
	}
	if cfg.GitLab.Project != "42" {
		t.Errorf("Project = %q, want 42", cfg.GitLab.Project)
	}
}

func TestApplyEnvDoesNotOverwrite(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "env-token")
	t.Setenv("CI_SERVER_URL", "https://env.example.com")
	t.Setenv("CI_PROJECT_ID", "99")

	cfg := defaults()
	cfg.GitLab.Token = "config-token"
	cfg.GitLab.URL = "https://config.example.com"
	cfg.GitLab.Project = "config-project"
	cfg.ApplyEnv()

	if cfg.GitLab.Token != "config-token" {
		t.Errorf("Token overwritten: got %q", cfg.GitLab.Token)
	}
	if cfg.GitLab.URL != "https://config.example.com" {
		t.Errorf("URL overwritten: got %q", cfg.GitLab.URL)
	}
	if cfg.GitLab.Project != "config-project" {
		t.Errorf("Project overwritten: got %q", cfg.GitLab.Project)
	}
}

func TestApplyEnvProjectPathFallback(t *testing.T) {
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("CI_PROJECT_PATH", "mygroup/myapp")

	cfg := defaults()
	cfg.ApplyEnv()

	if cfg.GitLab.Project != "mygroup/myapp" {
		t.Errorf("Project = %q, want mygroup/myapp (from CI_PROJECT_PATH)", cfg.GitLab.Project)
	}
}

func TestLoadPartialOverride(t *testing.T) {
	dir := t.TempDir()
	// Only override tag_prefix — commit_message should keep its default
	content := "git:\n  tag_prefix: \"\"\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Git.TagPrefix != "" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Git.TagPrefix, "")
	}
	if cfg.Git.CommitMessage != defaults().Git.CommitMessage {
		t.Errorf("CommitMessage = %q, want default", cfg.Git.CommitMessage)
	}
}
