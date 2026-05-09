package main

import (
	"encoding/json"
	"net/url"
	"os"
	"runtime"
)

const ConfigFile = "config.json"

type LlamaLocal struct {
	Enabled     bool   `json:"enabled"`
	Endpoint    string `json:"endpoint"`   // e.g. "http://localhost:11434"
	ModelPath   string `json:"model_path"`
	LlamaBin    string `json:"llama_bin"`
	ContextSize int    `json:"context_size"`
}

type LlamaRemote struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"` // e.g. "http://192.168.1.32:11434"
}

type Config struct {
	HTTPHost    string      `json:"http_host"`
	HTTPPort    string      `json:"http_port"`
	HTTPSPort   string      `json:"https_port"`
	LlamaLocal  LlamaLocal  `json:"llama_local"`
	LlamaRemote LlamaRemote `json:"llama_remote"`
	UploadDir   string      `json:"upload_dir"`
}

// IsRemote returns true when llama_remote.enabled is set.
// Remote always wins if enabled; local is the fallback.
func (c *Config) IsRemote() bool {
	return c.LlamaRemote.Enabled
}

// ActiveEndpoint returns the base URL of the active llama-server.
func (c *Config) ActiveEndpoint() string {
	if c.IsRemote() {
		return c.LlamaRemote.Endpoint
	}
	return c.LlamaLocal.Endpoint
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}

	// Defaults for local mode
	if cfg.LlamaLocal.Endpoint == "" {
		cfg.LlamaLocal.Endpoint = "http://localhost:11434"
	}
	if cfg.LlamaLocal.ContextSize == 0 {
		cfg.LlamaLocal.ContextSize = 4096
	}

	// Auto-select binary based on current OS
	if cfg.LlamaLocal.LlamaBin == "" || cfg.LlamaLocal.LlamaBin == "bin/linux/llama-server" {
		switch runtime.GOOS {
		case "windows":
			cfg.LlamaLocal.LlamaBin = `bin\windows\llama-server.exe`
		default:
			cfg.LlamaLocal.LlamaBin = "bin/linux/llama-server"
		}
	}

	return cfg, nil
}

// Save writes the current config back to the JSON file.
func (c *Config) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

// EndpointPort extracts the port from an endpoint URL string.
func endpointPort(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Port()
}
