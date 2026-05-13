package main

import (
	"encoding/json"
	"net/url"
	"os"
	"runtime"
)

const (
	ConfigFile       = "config.json"
	GoldenConfigFile = "config.gl" // File distribuito con gli aggiornamenti
)

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
	Endpoint        string `json:"endpoint"`
	ModelPath       string `json:"model_path"`
	MmprojPath      string `json:"mmproj_path"`
	VisionEnabled   bool   `json:"vision_enabled"`
	LlamaBin        string `json:"llama_bin"`
	LlamaBinVersion string `json:"llama_bin_version"`
	LlamaBinURL     string `json:"llama_bin_url"`
	ContextSize     int    `json:"context_size"`
}

type LlamaRemote struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
}

type Config struct {
	HTTPHost              string      `json:"http_host"`
	HTTPPort              string      `json:"http_port"`
	HTTPSPort             string      `json:"https_port"`
	LlamaLocal            LlamaLocal  `json:"llama_local"`
	LlamaRemote           LlamaRemote `json:"llama_remote"`
	ModelURLs             ModelURLs   `json:"model_urls"`
	UploadDir             string      `json:"upload_dir"`
	SysinfoRefreshSeconds int         `json:"sysinfo_refresh_seconds"`
}

func (c *Config) IsRemote() bool {
	return c.LlamaRemote.Enabled
}

func (c *Config) ActiveEndpoint() string {
	if c.IsRemote() {
		return c.LlamaRemote.Endpoint
	}
	return c.LlamaLocal.Endpoint
}

func mergeJSON(golden, user map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Partiamo dai valori del Golden
	for k, v := range golden {
		result[k] = v
	}

	for k, userVal := range user {
		// 1. Se il valore utente è nil, saltiamo
		if userVal == nil {
			continue
		}

		// 2. Gestione Ricorsiva per le mappe (LlamaLocal, ModelURLs, etc.)
		if goldenMap, ok := result[k].(map[string]interface{}); ok {
			if userMap, ok := userVal.(map[string]interface{}); ok {
				result[k] = mergeJSON(goldenMap, userMap)
				continue
			}
		}

		// 3. LOGICA DI SOVRASCRITTURA CRITICA
		switch v := userVal.(type) {
		case string:
			if v != "" { // Sovrascrive solo se la stringa non è vuota
				result[k] = v
			}
		case float64:
			if v != 0 { // Sovrascrive se il numero è diverso da zero
				result[k] = v
			}
		case bool:
			// Per i boolean dobbiamo sovrascrivere sempre,
			// altrimenti non potresti mai passare da true a false
			result[k] = v
		default:
			result[k] = v
		}
	}
	return result
}

func loadConfig(path string) (*Config, error) {
	var goldenData, userData map[string]interface{}

	// 1. Carica Golden Data se esiste
	if f, err := os.Open(GoldenConfigFile); err == nil {
		json.NewDecoder(f).Decode(&goldenData)
		f.Close()
	}

	// 2. Carica User Config se esiste
	configExists := false
	if f, err := os.Open(path); err == nil {
		configExists = true
		json.NewDecoder(f).Decode(&userData)
		f.Close()
	}

	var finalData map[string]interface{}
	shouldUpdateFile := false

	// 3. Logica di Merge
	if goldenData != nil {
		if configExists {
			finalData = mergeJSON(goldenData, userData)
		} else {
			finalData = goldenData
		}
		shouldUpdateFile = true
	} else if configExists {
		finalData = userData
	} else {
		return nil, os.ErrNotExist
	}

	// 4. Decode in Struct
	cfg := &Config{}
	tmp, _ := json.Marshal(finalData)
	if err := json.Unmarshal(tmp, cfg); err != nil {
		return nil, err
	}

	// 5. Applica Defaults (Commentato come richiesto)
	// cfg = applyConfigDefaults(cfg)

	// 6. Salvataggio e rimozione Golden
	if shouldUpdateFile {
		if err := cfg.Save(path); err == nil {
			os.Remove(GoldenConfigFile)
		}
	}

	return cfg, nil
}

func applyConfigDefaults(cfg *Config) *Config {
	if cfg.LlamaLocal.Endpoint == "" {
		cfg.LlamaLocal.Endpoint = "http://localhost:11434"
	}
	if cfg.LlamaLocal.ContextSize == 0 {
		cfg.LlamaLocal.ContextSize = 4096
	}
	if cfg.SysinfoRefreshSeconds == 0 {
		cfg.SysinfoRefreshSeconds = 5
	}

	if cfg.LlamaLocal.LlamaBin == "" {
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

	return cfg
}

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

func endpointPort(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Port()
}
