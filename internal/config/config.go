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
			TagPrefix:     "v",
			BranchPattern: branch.DefaultBranchPattern,
			CommitMessage: "chore(release): {version} [skip ci]",
		},
		Maven: MavenConfig{
			PomPath: "pom.xml",
		},
	}
}

// Load reads .releaser.yml from dir and merges it over the defaults.
// Missing file is not an error — defaults are returned as-is.
func Load(dir string) (Config, error) {
	cfg := defaults()

	data, err := os.ReadFile(filepath.Join(dir, filename))
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", filename, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", filename, err)
	}

	return cfg, nil
}

// ApplyEnv fills empty GitLab fields from the standard GitLab CI environment variables.
// Values already set in the config file are never overwritten.
func (c *Config) ApplyEnv() {
	if c.GitLab.Token == "" {
		c.GitLab.Token = os.Getenv("GITLAB_TOKEN")
	}
	if c.GitLab.URL == "" {
		// CI_SERVER_URL is the cleanest source ("https://gitlab.example.com")
		c.GitLab.URL = os.Getenv("CI_SERVER_URL")
	}
	if c.GitLab.Project == "" {
		// Prefer numeric ID; fall back to namespace/project path
		if id := os.Getenv("CI_PROJECT_ID"); id != "" {
			c.GitLab.Project = id
		} else {
			c.GitLab.Project = os.Getenv("CI_PROJECT_PATH")
		}
	}
}
