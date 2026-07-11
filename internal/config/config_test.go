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
	if cfg.Git.TagPrefix != "" {
		t.Errorf("TagPrefix = %q, want %q", cfg.Git.TagPrefix, "")
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

func TestLoadWithSourcesFullConfig(t *testing.T) {
	dir := t.TempDir()
	content := `
git:
  tag_prefix: "v"
  branch_pattern: "^release/(\\d+)$"
  commit_message: "release {version}"
  author_name: "Bot"
  author_email: "bot@example.com"
  releasable_types: ["fix", "feat"]
  bump_rules:
    breaking: "minor"
    feat: "patch"
    fix: "patch"
maven:
  pom_path: "sub/pom.xml"
  pom_paths:
    - "a/pom.xml"
    - "b/pom.xml"
node:
  package_json: "frontend/package.json"
  package_jsons:
    - "pkg-a/package.json"
    - "pkg-b/package.json"
gitlab:
  url: "https://gitlab.example.com"
  token: "gitlab-token"
  project: "42"
github:
  token: "github-token"
  repo: "owner/repo"
`
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, src, err := LoadWithSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	wantConfigFile := []string{
		"git.tag_prefix", "git.branch_pattern", "git.commit_message",
		"git.author_name", "git.author_email", "git.releasable_types",
		"git.bump_rules.breaking", "git.bump_rules.feat", "git.bump_rules.fix",
		"maven.pom_path", "maven.pom_paths",
		"node.package_json", "node.package_jsons",
		"gitlab.url", "gitlab.token", "gitlab.project",
		"github.token", "github.repo",
	}
	for _, key := range wantConfigFile {
		if got := src[key]; got != "config file" {
			t.Errorf("src[%q] = %q, want %q", key, got, "config file")
		}
	}
}

func TestEffectivePomPaths(t *testing.T) {
	cases := []struct {
		name string
		cfg  MavenConfig
		want []string
	}{
		{"default", MavenConfig{PomPath: "pom.xml"}, []string{"pom.xml"}},
		{"single override", MavenConfig{PomPath: "sub/pom.xml"}, []string{"sub/pom.xml"}},
		{"multi overrides single", MavenConfig{PomPath: "pom.xml", PomPaths: []string{"a/pom.xml", "b/pom.xml"}}, []string{"a/pom.xml", "b/pom.xml"}},
		{"empty falls back to default", MavenConfig{}, []string{"pom.xml"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.cfg.EffectivePomPaths()
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestNodeEffectivePaths(t *testing.T) {
	cases := []struct {
		name string
		cfg  NodeConfig
		want []string
	}{
		{"empty — opt-in, skip by default", NodeConfig{}, nil},
		{"single", NodeConfig{PackageJSON: "package.json"}, []string{"package.json"}},
		{"multi overrides single", NodeConfig{PackageJSON: "package.json", PackageJSONs: []string{"a/package.json", "b/package.json"}}, []string{"a/package.json", "b/package.json"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.cfg.EffectivePaths()
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("[%d] got %q, want %q", i, got[i], c.want[i])
				}
			}
		})
	}
}

func TestApplyEnvWithSourcesCIProjectID(t *testing.T) {
	t.Setenv("CI_PROJECT_ID", "123")
	t.Setenv("CI_PROJECT_PATH", "")

	cfg := defaults()
	src := defaultSources()
	cfg.ApplyEnvWithSources(src)

	if src["gitlab.project"] != "env: CI_PROJECT_ID" {
		t.Errorf("src[gitlab.project] = %q, want %q", src["gitlab.project"], "env: CI_PROJECT_ID")
	}
}

func TestApplyEnvWithSourcesGitHubToken(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "ghtoken")

	cfg := defaults()
	src := defaultSources()
	cfg.ApplyEnvWithSources(src)

	if src["github.token"] != "env: GITHUB_TOKEN" {
		t.Errorf("src[github.token] = %q, want %q", src["github.token"], "env: GITHUB_TOKEN")
	}
}

func TestApplyEnvWithSourcesCIProjectPath(t *testing.T) {
	t.Setenv("CI_PROJECT_ID", "")
	t.Setenv("CI_PROJECT_PATH", "group/project")

	cfg := defaults()
	src := defaultSources()
	cfg.ApplyEnvWithSources(src)

	if src["gitlab.project"] != "env: CI_PROJECT_PATH" {
		t.Errorf("src[gitlab.project] = %q, want %q", src["gitlab.project"], "env: CI_PROJECT_PATH")
	}
}

func TestApplyEnvWithSourcesGitLabToken(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "mytoken")

	cfg := defaults()
	src := defaultSources()
	cfg.ApplyEnvWithSources(src)

	if src["gitlab.token"] != "env: GITLAB_TOKEN" {
		t.Errorf("src[gitlab.token] = %q, want %q", src["gitlab.token"], "env: GITLAB_TOKEN")
	}
}

func TestApplyEnvWithSourcesCIServerURL(t *testing.T) {
	t.Setenv("CI_SERVER_URL", "https://gitlab.example.com")

	cfg := defaults()
	src := defaultSources()
	cfg.ApplyEnvWithSources(src)

	if src["gitlab.url"] != "env: CI_SERVER_URL" {
		t.Errorf("src[gitlab.url] = %q, want %q", src["gitlab.url"], "env: CI_SERVER_URL")
	}
}

func TestGradleEffectiveBuildFiles(t *testing.T) {
	// Neither set → nil (opt-in)
	if got := (GradleConfig{}).EffectiveBuildFiles(); got != nil {
		t.Errorf("empty config: got %v, want nil", got)
	}
	// BuildFile only
	if got := (GradleConfig{BuildFile: "build.gradle"}).EffectiveBuildFiles(); len(got) != 1 || got[0] != "build.gradle" {
		t.Errorf("BuildFile only: got %v", got)
	}
	// BuildFiles wins over BuildFile
	g := GradleConfig{BuildFile: "build.gradle", BuildFiles: []string{"a/build.gradle", "b/build.gradle"}}
	if got := g.EffectiveBuildFiles(); len(got) != 2 || got[0] != "a/build.gradle" {
		t.Errorf("BuildFiles priority: got %v", got)
	}
}

func TestLoadGradleSources(t *testing.T) {
	dir := t.TempDir()
	content := "gradle:\n  build_file: \"build.gradle\"\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, src, err := LoadWithSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if src["gradle.build_file"] != "config file" {
		t.Errorf("src[gradle.build_file] = %q, want %q", src["gradle.build_file"], "config file")
	}
}

func TestLoadGradleBuildFilesSources(t *testing.T) {
	dir := t.TempDir()
	content := "gradle:\n  build_files:\n    - \"a/build.gradle\"\n    - \"b/build.gradle\"\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, src, err := LoadWithSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if src["gradle.build_files"] != "config file" {
		t.Errorf("src[gradle.build_files] = %q, want %q", src["gradle.build_files"], "config file")
	}
}

func TestPythonEffectivePaths(t *testing.T) {
	if got := (PythonConfig{}).EffectivePaths(); got != nil {
		t.Errorf("empty config: got %v, want nil", got)
	}
	if got := (PythonConfig{PyprojectTOML: "pyproject.toml"}).EffectivePaths(); len(got) != 1 || got[0] != "pyproject.toml" {
		t.Errorf("single path: got %v", got)
	}
	p := PythonConfig{PyprojectTOML: "pyproject.toml", PyprojectTOMLs: []string{"a/pyproject.toml", "b/pyproject.toml"}}
	if got := p.EffectivePaths(); len(got) != 2 || got[0] != "a/pyproject.toml" {
		t.Errorf("multi-path priority: got %v", got)
	}
}

func TestLoadPythonSources(t *testing.T) {
	dir := t.TempDir()
	content := "python:\n  pyproject_toml: \"pyproject.toml\"\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, src, err := LoadWithSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if src["python.pyproject_toml"] != "config file" {
		t.Errorf("src[python.pyproject_toml] = %q, want %q", src["python.pyproject_toml"], "config file")
	}
}

func TestLoadPythonPathsSources(t *testing.T) {
	dir := t.TempDir()
	content := "python:\n  pyproject_tomls:\n    - \"a/pyproject.toml\"\n    - \"b/pyproject.toml\"\n"
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, src, err := LoadWithSources(dir)
	if err != nil {
		t.Fatal(err)
	}
	if src["python.pyproject_tomls"] != "config file" {
		t.Errorf("src[python.pyproject_tomls] = %q, want %q", src["python.pyproject_tomls"], "config file")
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
