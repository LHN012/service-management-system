package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ScanIntervalSeconds int `yaml:"scanIntervalSeconds"`
	StopTimeoutSeconds  int `yaml:"stopTimeoutSeconds"`
	LogRetentionDays    int `yaml:"logRetentionDays"`
}

func Default() Config {
	return Config{ScanIntervalSeconds: 60, StopTimeoutSeconds: 15, LogRetentionDays: 30}
}

func Load(root string) (Config, error) {
	config := Default()
	data, err := os.ReadFile(filepath.Join(root, "conf", "app.yml"))
	if errors.Is(err, os.ErrNotExist) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("parse conf/app.yml: %w", err)
	}
	if config.ScanIntervalSeconds < 10 {
		return config, fmt.Errorf("scanIntervalSeconds must be at least 10")
	}
	if config.StopTimeoutSeconds < 1 {
		return config, fmt.Errorf("stopTimeoutSeconds must be positive")
	}
	return config, nil
}

func Save(root string, value Config) error {
	if value.ScanIntervalSeconds < 10 {
		return fmt.Errorf("scanIntervalSeconds must be at least 10")
	}
	if value.StopTimeoutSeconds < 1 {
		return fmt.Errorf("stopTimeoutSeconds must be positive")
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	dir := filepath.Join(root, "conf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "app.yml")
	tmp, err := os.CreateTemp(dir, ".app-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err == nil {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, path)
}
