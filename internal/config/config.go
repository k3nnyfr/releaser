package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"git.k3nny.fr/releaser/internal/branch"
)

const filename = ".releaser.yml"

type Config struct {
	Git    GitConfig    `yaml:"git"`
	Maven  MavenConfig  `yaml:"maven"`
	Node   NodeConfig   `yaml:"node"`
	Gradle  GradleConfig  `yaml:"gradle"`
	Python  PythonConfig  `yaml:"python"`
	GitLab  GitLabConfig  `yaml:"gitlab"`
	GitHub GitHubConfig `yaml:"github"`
	Notify NotifyConfig `yaml:"notify"`
}

type GitConfig struct {
	TagPrefix       string         `yaml:"tag_prefix"`
	BranchPattern   string         `yaml:"branch_pattern"`
	CommitMessage   string         `yaml:"commit_message"`
	AuthorName      string         `yaml:"author_name"`
	AuthorEmail     string         `yaml:"author_email"`
	ReleasableTypes []string       `yaml:"releasable_types"`
	BumpRules       BumpRulesConfig `yaml:"bump_rules"`
}

// BumpRulesConfig controls what version component each commit type bumps.
// Valid values: "patch" (default) or "minor".
type BumpRulesConfig struct {
	Breaking string `yaml:"breaking"`
	Feat     string `yaml:"feat"`
	Fix      string `yaml:"fix"`
}

type MavenConfig struct {
	PomPath  string   `yaml:"pom_path"`  // single path (default: "pom.xml")
	PomPaths []string `yaml:"pom_paths"` // multiple paths; overrides PomPath when set
}

// EffectivePomPaths returns the list of pom.xml paths to process.
// PomPaths takes precedence over PomPath; falls back to ["pom.xml"].
func (m MavenConfig) EffectivePomPaths() []string {
	if len(m.PomPaths) > 0 {
		return m.PomPaths
	}
	if m.PomPath != "" {
		return []string{m.PomPath}
	}
	return []string{"pom.xml"}
}

type NodeConfig struct {
	PackageJSON  string   `yaml:"package_json"`  // single path
	PackageJSONs []string `yaml:"package_jsons"` // multiple paths; overrides PackageJSON when set
}

// EffectivePaths returns the list of package.json paths to process.
// Returns nil when no node paths are configured (node processing is opt-in).
func (n NodeConfig) EffectivePaths() []string {
	if len(n.PackageJSONs) > 0 {
		return n.PackageJSONs
	}
	if n.PackageJSON != "" {
		return []string{n.PackageJSON}
	}
	return nil
}

type GradleConfig struct {
	BuildFile  string   `yaml:"build_file"`  // single path (opt-in, no default)
	BuildFiles []string `yaml:"build_files"` // multiple paths; overrides BuildFile
}

// EffectiveBuildFiles returns the list of Gradle build file paths to process.
// Returns nil when no gradle paths are configured (gradle processing is opt-in).
func (g GradleConfig) EffectiveBuildFiles() []string {
	if len(g.BuildFiles) > 0 {
		return g.BuildFiles
	}
	if g.BuildFile != "" {
		return []string{g.BuildFile}
	}
	return nil
}

type PythonConfig struct {
	PyprojectTOML  string   `yaml:"pyproject_toml"`  // single path (opt-in, no default)
	PyprojectTOMLs []string `yaml:"pyproject_tomls"` // multiple paths; overrides PyprojectTOML
}

// EffectivePaths returns the list of pyproject.toml paths to process.
// Returns nil when no python paths are configured (python processing is opt-in).
func (p PythonConfig) EffectivePaths() []string {
	if len(p.PyprojectTOMLs) > 0 {
		return p.PyprojectTOMLs
	}
	if p.PyprojectTOML != "" {
		return []string{p.PyprojectTOML}
	}
	return nil
}

type GitLabConfig struct {
	URL     string `yaml:"url"`
	Token   string `yaml:"token"`
	Project string `yaml:"project"`
}

type GitHubConfig struct {
	Token string `yaml:"token"`
	Repo  string `yaml:"repo"` // "owner/repo"
}

// NotifyConfig configures best-effort release notifications. Every field is
// opt-in — a target is only used when its required fields are non-empty.
type NotifyConfig struct {
	SlackWebhookURL      string `yaml:"slack_webhook_url"`
	TeamsWebhookURL      string `yaml:"teams_webhook_url"`
	GoogleChatWebhookURL string `yaml:"google_chat_webhook_url"`
	TelegramBotToken     string `yaml:"telegram_bot_token"`
	TelegramChatID       string `yaml:"telegram_chat_id"`
	WebhookURL           string `yaml:"webhook_url"`
}

func defaults() Config {
	return Config{
		Git: GitConfig{
			TagPrefix:     "",
			BranchPattern: branch.DefaultBranchPattern,
			CommitMessage: "chore(release): {version} [skip ci]",
		},
		Maven: MavenConfig{
			PomPath: "pom.xml",
		},
	}
}

// Sources records where each config value came from.
// Keys are "section.field" (e.g. "git.tag_prefix").
// Values are one of: "default", "config file", "env: VARNAME", "flag: --flag-name".
type Sources map[string]string

func defaultSources() Sources {
	return Sources{
		"git.tag_prefix":          "default",
		"git.branch_pattern":      "default",
		"git.commit_message":      "default",
		"git.author_name":         "default",
		"git.author_email":        "default",
		"git.releasable_types":    "default",
		"git.bump_rules.breaking": "default",
		"git.bump_rules.feat":     "default",
		"git.bump_rules.fix":      "default",
		"maven.pom_path":          "default",
		"maven.pom_paths":         "default",
		"node.package_json":       "default",
		"node.package_jsons":      "default",
		"gradle.build_file":        "default",
		"gradle.build_files":       "default",
		"python.pyproject_toml":    "default",
		"python.pyproject_tomls":   "default",
		"gitlab.url":               "default",
		"gitlab.token":            "default",
		"gitlab.project":          "default",
		"github.token":            "default",
		"github.repo":             "default",
		"notify.slack_webhook_url":       "default",
		"notify.teams_webhook_url":       "default",
		"notify.google_chat_webhook_url": "default",
		"notify.telegram_bot_token":      "default",
		"notify.telegram_chat_id":        "default",
		"notify.webhook_url":             "default",
	}
}

// Load reads .releaser.yml from dir and merges it over the defaults.
// Missing file is not an error — defaults are returned as-is.
func Load(dir string) (Config, error) {
	cfg, _, err := LoadWithSources(dir)
	return cfg, err
}

// LoadWithSources is like Load but also returns a Sources map recording where each
// value came from ("default" or "config file").
func LoadWithSources(dir string) (Config, Sources, error) {
	cfg := defaults()
	src := defaultSources()

	data, err := os.ReadFile(filepath.Join(dir, filename))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, src, nil
	}
	if err != nil {
		return cfg, src, fmt.Errorf("read %s: %w", filename, err)
	}

	// Unmarshal into cfg (merges over defaults).
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, src, fmt.Errorf("parse %s: %w", filename, err)
	}

	// Detect which fields the file explicitly set by unmarshaling into a zero overlay.
	var overlay Config
	_ = yaml.Unmarshal(data, &overlay)
	if overlay.Git.TagPrefix != "" {
		src["git.tag_prefix"] = "config file"
	}
	if overlay.Git.BranchPattern != "" {
		src["git.branch_pattern"] = "config file"
	}
	if overlay.Git.CommitMessage != "" {
		src["git.commit_message"] = "config file"
	}
	if overlay.Git.AuthorName != "" {
		src["git.author_name"] = "config file"
	}
	if overlay.Git.AuthorEmail != "" {
		src["git.author_email"] = "config file"
	}
	if len(overlay.Git.ReleasableTypes) > 0 {
		src["git.releasable_types"] = "config file"
	}
	if overlay.Git.BumpRules.Breaking != "" {
		src["git.bump_rules.breaking"] = "config file"
	}
	if overlay.Git.BumpRules.Feat != "" {
		src["git.bump_rules.feat"] = "config file"
	}
	if overlay.Git.BumpRules.Fix != "" {
		src["git.bump_rules.fix"] = "config file"
	}
	if overlay.Maven.PomPath != "" {
		src["maven.pom_path"] = "config file"
	}
	if len(overlay.Maven.PomPaths) > 0 {
		src["maven.pom_paths"] = "config file"
	}
	if overlay.Node.PackageJSON != "" {
		src["node.package_json"] = "config file"
	}
	if len(overlay.Node.PackageJSONs) > 0 {
		src["node.package_jsons"] = "config file"
	}
	if overlay.Gradle.BuildFile != "" {
		src["gradle.build_file"] = "config file"
	}
	if len(overlay.Gradle.BuildFiles) > 0 {
		src["gradle.build_files"] = "config file"
	}
	if overlay.Python.PyprojectTOML != "" {
		src["python.pyproject_toml"] = "config file"
	}
	if len(overlay.Python.PyprojectTOMLs) > 0 {
		src["python.pyproject_tomls"] = "config file"
	}
	if overlay.GitLab.URL != "" {
		src["gitlab.url"] = "config file"
	}
	if overlay.GitLab.Token != "" {
		src["gitlab.token"] = "config file"
	}
	if overlay.GitLab.Project != "" {
		src["gitlab.project"] = "config file"
	}
	if overlay.GitHub.Token != "" {
		src["github.token"] = "config file"
	}
	if overlay.GitHub.Repo != "" {
		src["github.repo"] = "config file"
	}
	if overlay.Notify.SlackWebhookURL != "" {
		src["notify.slack_webhook_url"] = "config file"
	}
	if overlay.Notify.TeamsWebhookURL != "" {
		src["notify.teams_webhook_url"] = "config file"
	}
	if overlay.Notify.GoogleChatWebhookURL != "" {
		src["notify.google_chat_webhook_url"] = "config file"
	}
	if overlay.Notify.TelegramBotToken != "" {
		src["notify.telegram_bot_token"] = "config file"
	}
	if overlay.Notify.TelegramChatID != "" {
		src["notify.telegram_chat_id"] = "config file"
	}
	if overlay.Notify.WebhookURL != "" {
		src["notify.webhook_url"] = "config file"
	}

	return cfg, src, nil
}

// ApplyEnv fills empty GitLab and GitHub fields from environment variables.
// Values already set in the config file are never overwritten.
func (c *Config) ApplyEnv() {
	c.ApplyEnvWithSources(nil)
}

// ApplyEnvWithSources is like ApplyEnv but records the env var name in src for each
// field it fills. src may be nil.
func (c *Config) ApplyEnvWithSources(src Sources) {
	if c.GitLab.Token == "" {
		if v := os.Getenv("GITLAB_TOKEN"); v != "" {
			c.GitLab.Token = v
			if src != nil {
				src["gitlab.token"] = "env: GITLAB_TOKEN"
			}
		}
	}
	if c.GitLab.URL == "" {
		if v := os.Getenv("CI_SERVER_URL"); v != "" {
			c.GitLab.URL = v
			if src != nil {
				src["gitlab.url"] = "env: CI_SERVER_URL"
			}
		}
	}
	if c.GitLab.Project == "" {
		if id := os.Getenv("CI_PROJECT_ID"); id != "" {
			c.GitLab.Project = id
			if src != nil {
				src["gitlab.project"] = "env: CI_PROJECT_ID"
			}
		} else if p := os.Getenv("CI_PROJECT_PATH"); p != "" {
			c.GitLab.Project = p
			if src != nil {
				src["gitlab.project"] = "env: CI_PROJECT_PATH"
			}
		}
	}
	if c.GitHub.Token == "" {
		if v := os.Getenv("GITHUB_TOKEN"); v != "" {
			c.GitHub.Token = v
			if src != nil {
				src["github.token"] = "env: GITHUB_TOKEN"
			}
		}
	}
	if c.Notify.SlackWebhookURL == "" {
		if v := os.Getenv("SLACK_WEBHOOK_URL"); v != "" {
			c.Notify.SlackWebhookURL = v
			if src != nil {
				src["notify.slack_webhook_url"] = "env: SLACK_WEBHOOK_URL"
			}
		}
	}
	if c.Notify.TeamsWebhookURL == "" {
		if v := os.Getenv("TEAMS_WEBHOOK_URL"); v != "" {
			c.Notify.TeamsWebhookURL = v
			if src != nil {
				src["notify.teams_webhook_url"] = "env: TEAMS_WEBHOOK_URL"
			}
		}
	}
	if c.Notify.GoogleChatWebhookURL == "" {
		if v := os.Getenv("GOOGLE_CHAT_WEBHOOK_URL"); v != "" {
			c.Notify.GoogleChatWebhookURL = v
			if src != nil {
				src["notify.google_chat_webhook_url"] = "env: GOOGLE_CHAT_WEBHOOK_URL"
			}
		}
	}
	if c.Notify.TelegramBotToken == "" {
		if v := os.Getenv("TELEGRAM_BOT_TOKEN"); v != "" {
			c.Notify.TelegramBotToken = v
			if src != nil {
				src["notify.telegram_bot_token"] = "env: TELEGRAM_BOT_TOKEN"
			}
		}
	}
	if c.Notify.TelegramChatID == "" {
		if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
			c.Notify.TelegramChatID = v
			if src != nil {
				src["notify.telegram_chat_id"] = "env: TELEGRAM_CHAT_ID"
			}
		}
	}
	if c.Notify.WebhookURL == "" {
		if v := os.Getenv("RELEASER_WEBHOOK_URL"); v != "" {
			c.Notify.WebhookURL = v
			if src != nil {
				src["notify.webhook_url"] = "env: RELEASER_WEBHOOK_URL"
			}
		}
	}
}
