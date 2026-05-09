package main

import (
	"encoding/json"
	"os"
	"runtime"
)

const ConfigFile = "config.json"

type Config struct {
	HTTPPort    string `json:"http_port"`
	HTTPSPort   string `json:"https_port"`
	LlamaPort   string `json:"llama_port"`
	ModelPath   string `json:"model_path"`
	LlamaBin    string `json:"llama_bin"`
	UploadDir   string `json:"upload_dir"`
	ContextSize int    `json:"context_size"`
	Hostname    string `json:"hostname"` // custom hostname for QR URL and cert SAN
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

	// Auto-select binary based on current OS
	if cfg.LlamaBin == "" || cfg.LlamaBin == "bin/linux/llama-server" {
		switch runtime.GOOS {
		case "windows":
			cfg.LlamaBin = `bin\windows\llama-server.exe`
		default:
			cfg.LlamaBin = "bin/linux/llama-server"
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
