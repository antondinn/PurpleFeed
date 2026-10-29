package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const purpfeedConfig = ".purpfeed.config"

type Config struct {
	DBURL string `json:"db_url"`
	VaultPath string `json:"vault_path"`
	PullPath string `json:"inbox_path"`
	SavedPath string `json:"saved_note_path"`
}

func getConfigPath() (string, error){
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, purpfeedConfig), nil
}

func Read() (Config, error) {
	path, err := getConfigPath()
	if err != nil {
		return Config{}, err
	}

	info, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(info, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) Save() error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}

	info, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, info, 0600)
}

func (c *Config) SetVaultPath(path string) error {
	c.VaultPath = path
	return c.Save()
}

func (c *Config) SetPullPath(path string) error {
	c.PullPath = path
	return c.Save()
}

func (c *Config) SetSavedPath(path string) error {
	c.SavedPath = path
	return c.Save()
}

func (c *Config) SetDBURL(dbURL string) error {
	c.DBURL = dbURL
	return c.Save()
}