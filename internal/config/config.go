package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIKey        string `yaml:"api_key,omitempty"`
	OpRef         string `yaml:"op_ref,omitempty"`
	Model         string `yaml:"model,omitempty"`
	RedactSecrets *bool  `yaml:"redact_secrets,omitempty"`
}

// RedactSecretsEnabled reports whether sensitive filenames should be redacted
// from the working-directory context sent to the model. Defaults to true when
// the field is unset.
func (c Config) RedactSecretsEnabled() bool {
	if c.RedactSecrets == nil {
		return true
	}
	return *c.RedactSecrets
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ai", "config.yaml"), nil
}

func Load() (Config, error) {
	p, err := Path()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", p, err)
	}
	return c, nil
}

func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

// ResolveOpRef calls `op read <ref>` to fetch a secret from 1Password. Returns
// a friendly error if the op CLI is missing or the user isn't signed in.
func ResolveOpRef(ctx context.Context, ref string) (string, error) {
	if _, err := exec.LookPath("op"); err != nil {
		return "", errors.New("1Password CLI (op) not found in PATH — install it or unset op_ref in config")
	}
	cmd := exec.CommandContext(ctx, "op", "read", ref)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("op read %s: %s", ref, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("op read %s: %w", ref, err)
	}
	key := strings.TrimSpace(string(out))
	if key == "" {
		return "", fmt.Errorf("op read %s returned empty value", ref)
	}
	return key, nil
}
