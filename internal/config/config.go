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
	GitLab GitLabConfig `yaml:"gitlab"`
}

type GitConfig struct {
	TagPrefix     string `yaml:"tag_prefix"`
	BranchPattern string `yaml:"branch_pattern"`
	CommitMessage string `yaml:"commit_message"`
	AuthorName    string `yaml:"author_name"`
	AuthorEmail   string `yaml:"author_email"`
}

type MavenConfig struct {
	PomPath string `yaml:"pom_path"`
}

type GitLabConfig struct {
	URL     string `yaml:"url"`
	Token   string `yaml:"token"`
	Project string `yaml:"project"`
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
		"git.tag_prefix":      "default",
		"git.branch_pattern":  "default",
		"git.commit_message":  "default",
		"git.author_name":     "default",
		"git.author_email":    "default",
		"maven.pom_path":      "default",
		"gitlab.url":          "default",
		"gitlab.token":        "default",
		"gitlab.project":      "default",
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
	if overlay.Maven.PomPath != "" {
		src["maven.pom_path"] = "config file"
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

	return cfg, src, nil
}

// ApplyEnv fills empty GitLab fields from the standard GitLab CI environment variables.
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
		// CI_SERVER_URL is the cleanest source ("https://gitlab.example.com")
		if v := os.Getenv("CI_SERVER_URL"); v != "" {
			c.GitLab.URL = v
			if src != nil {
				src["gitlab.url"] = "env: CI_SERVER_URL"
			}
		}
	}
	if c.GitLab.Project == "" {
		// Prefer numeric ID; fall back to namespace/project path
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
}
