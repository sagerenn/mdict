package config

import (
	"encoding/json"
	"errors"
	"os"
	"time"
)

type Config struct {
	Listen          string        `json:"listen"`
	ReadTimeout     time.Duration `json:"read_timeout"`
	WriteTimeout    time.Duration `json:"write_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
	Log             LogConfig     `json:"log"`
	Dictionaries    []DictConfig  `json:"dictionaries"`
}

type LogConfig struct {
	Level string `json:"level"`
}

type DictConfig struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Delimiter string `json:"delimiter"`
	CaseFold  bool   `json:"case_fold"`
}

func Default() Config {
	return Config{
		Listen:          ":8080",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		Log: LogConfig{
			Level: "info",
		},
		Dictionaries: nil,
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, errors.New("config path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Listen == "" {
		cfg.Listen = ":8080"
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 5 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 30 * time.Second
	}
	if cfg.ShutdownTimeout == 0 {
		cfg.ShutdownTimeout = 10 * time.Second
	}
	return cfg, nil
}
