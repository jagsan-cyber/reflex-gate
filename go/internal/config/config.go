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
	JevPort     int    `json:"jev_port"`
	LlamaPort   int    `json:"llama_port"`
	Host        string `json:"host"`
	Lang        string `json:"lang"`
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
	c := Config{
		JevPort:   8090,
		LlamaPort: 8080,
		Host:      "0.0.0.0",
		Context:   8192,
		Lang:      "ja",
	}
	b, err := os.ReadFile(Path())
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	if c.JevPort <= 0 {
		c.JevPort = 8090
	}
	if c.LlamaPort <= 0 {
		c.LlamaPort = 8080
	}
	if c.Host == "" {
		c.Host = "0.0.0.0"
	}
	if c.Lang != "en" && c.Lang != "ja" {
		c.Lang = "ja"
	}
	return c
}

func (c Config) Save() error {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), b, 0o644)
}
