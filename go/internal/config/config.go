package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	LlamaServer string `json:"llama_server"`
	Model       string `json:"model"`
	Mode        string `json:"mode"` // auto | manual
	Context     int    `json:"context"`
	Backend     string `json:"backend"`
	GPULayers   int    `json:"gpu_layers"`
}

func Path() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

func DataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func Load() Config {
	var c Config
	b, err := os.ReadFile(Path())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	return c
}

func (c Config) Save() error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), b, 0o644)
}
