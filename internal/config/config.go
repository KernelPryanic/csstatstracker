package config

import (
	"encoding/json"
	"fmt"
	"os"
)

const DefaultConfigFile = "./csstatstracker.json"

// Hotkeys defines the keyboard shortcuts for each action
type Hotkeys struct {
	IncrementCT []string `json:"increment_ct"`
	DecrementCT []string `json:"decrement_ct"`
	IncrementT  []string `json:"increment_t"`
	DecrementT  []string `json:"decrement_t"`
	Reset       []string `json:"reset"`
	SelectCT    []string `json:"select_ct"`
	SelectT     []string `json:"select_t"`
}

// Config holds the application configuration
type Config struct {
	GameScore      int     `json:"game_score"`
	SoundEnabled   bool    `json:"sound_enabled"`
	SoundVolume    float64 `json:"sound_volume"`
	MinimizeToTray bool    `json:"minimize_to_tray"`
	Hotkeys        Hotkeys `json:"hotkeys"`
}

// Default returns the default configuration
// Hotkey defaults are platform-specific (see defaults_linux.go, defaults_windows.go)
func Default() *Config {
	return &Config{
		GameScore:      8,
		SoundEnabled:   true,
		SoundVolume:    1.0,
		MinimizeToTray: false,
		Hotkeys:        defaultHotkeys(),
	}
}

// Load reads the configuration from the specified file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Ensure hotkeys are set if missing
	if len(cfg.Hotkeys.IncrementCT) == 0 {
		def := Default()
		cfg.Hotkeys = def.Hotkeys
	}

	// Ensure sound volume is set if missing (0 means not set in config)
	if cfg.SoundVolume == 0 {
		cfg.SoundVolume = 1.0
	}

	return &cfg, nil
}

// Save writes the configuration to the specified file
func Save(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}
