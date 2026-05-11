package main

import (
	"encoding/json"
	"net/url"
	"os"
	"runtime"
)

const ConfigFile = "config.json"

// Default Gemma 4 model download URLs (Q4_K_M quantization, lmstudio-community).
const (
	DefaultModelURLe2b    = "https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q4_K_M.gguf"
	DefaultModelURLe4b    = "https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf"
	DefaultModelURLe31b   = "https://huggingface.co/lmstudio-community/gemma-4-E31B-it-GGUF/resolve/main/gemma-4-E31B-it-Q4_K_M.gguf"
	DefaultMmprojURLe2b   = "https://huggingface.co/lmstudio-community/gemma-4-E2B-it-GGUF/resolve/main/mmproj-gemma-4-E2B-it-BF16.gguf"
	DefaultMmprojURLe4b   = "https://huggingface.co/lmstudio-community/gemma-4-E4B-it-GGUF/resolve/main/mmproj-gemma-4-E4B-it-BF16.gguf"
	DefaultMmprojURLe31b  = "https://huggingface.co/lmstudio-community/gemma-4-E31B-it-GGUF/resolve/main/mmproj-gemma-4-E31B-it-BF16.gguf"
	DefaultModelPathE2B   = "models/gemma-4-e2b.gguf"
	DefaultModelPathE4B   = "models/gemma-4-e4b.gguf"
	DefaultModelPathE31B  = "models/gemma-4-e31b.gguf"
	DefaultMmprojPathE2B  = "models/mmproj-gemma-4-e2b.gguf"
	DefaultMmprojPathE4B  = "models/mmproj-gemma-4-e4b.gguf"
	DefaultMmprojPathE31B = "models/mmproj-gemma-4-e31b.gguf"
)

// ModelURLs holds overridable download URLs for the Gemma model variants.
type ModelURLs struct {
	E2B        string `json:"e2b"`
	E4B        string `json:"e4b"`
	E31B       string `json:"e31b"`
	MmprojE2B  string `json:"mmproj_e2b"`
	MmprojE4B  string `json:"mmproj_e4b"`
	MmprojE31B string `json:"mmproj_e31b"`
}

type LlamaLocal struct {
	Enabled         bool   `json:"enabled"`
	Endpoint        string `json:"endpoint"` // e.g. "http://localhost:11434"
	ModelPath       string `json:"model_path"`
	MmprojPath      string `json:"mmproj_path"`    // multimodal projector for vision
	VisionEnabled   bool   `json:"vision_enabled"` // whether to pass --mmproj to llama-server
	LlamaBin        string `json:"llama_bin"`
	LlamaBinVersion string `json:"llama_bin_version"` // active release tag, e.g. "b9095"
	ContextSize     int    `json:"context_size"`
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
	ModelURLs   ModelURLs   `json:"model_urls"`
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

	// Model URL defaults
	if cfg.ModelURLs.E2B == "" {
		cfg.ModelURLs.E2B = DefaultModelURLe2b
	}
	if cfg.ModelURLs.E4B == "" {
		cfg.ModelURLs.E4B = DefaultModelURLe4b
	}
	if cfg.ModelURLs.E31B == "" {
		cfg.ModelURLs.E31B = DefaultModelURLe31b
	}
	if cfg.ModelURLs.MmprojE2B == "" {
		cfg.ModelURLs.MmprojE2B = DefaultMmprojURLe2b
	}
	if cfg.ModelURLs.MmprojE4B == "" {
		cfg.ModelURLs.MmprojE4B = DefaultMmprojURLe4b
	}
	if cfg.ModelURLs.MmprojE31B == "" {
		cfg.ModelURLs.MmprojE31B = DefaultMmprojURLe31b
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
