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
	SwapTeams   []string `json:"swap_teams"`
}

// Config holds the application configuration
type Config struct {
	GameScore      int     `json:"game_score"`
	SoundEnabled   bool    `json:"sound_enabled"`
	SoundVolume    float64 `json:"sound_volume"`
	MinimizeToTray bool    `json:"minimize_to_tray"`
	Hotkeys        Hotkeys `json:"hotkeys"`
	StatsPeriod    string  `json:"stats_period"`
	StatsGroup     string  `json:"stats_group"`
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
		StatsPeriod:    "All Time",
		StatsGroup:     "By Day",
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

	// Ensure all hotkeys are set if missing (for app upgrades)
	def := Default()
	if len(cfg.Hotkeys.IncrementCT) == 0 {
		cfg.Hotkeys.IncrementCT = def.Hotkeys.IncrementCT
	}
	if len(cfg.Hotkeys.DecrementCT) == 0 {
		cfg.Hotkeys.DecrementCT = def.Hotkeys.DecrementCT
	}
	if len(cfg.Hotkeys.IncrementT) == 0 {
		cfg.Hotkeys.IncrementT = def.Hotkeys.IncrementT
	}
	if len(cfg.Hotkeys.DecrementT) == 0 {
		cfg.Hotkeys.DecrementT = def.Hotkeys.DecrementT
	}
	if len(cfg.Hotkeys.Reset) == 0 {
		cfg.Hotkeys.Reset = def.Hotkeys.Reset
	}
	if len(cfg.Hotkeys.SelectCT) == 0 {
		cfg.Hotkeys.SelectCT = def.Hotkeys.SelectCT
	}
	if len(cfg.Hotkeys.SelectT) == 0 {
		cfg.Hotkeys.SelectT = def.Hotkeys.SelectT
	}
	if len(cfg.Hotkeys.SwapTeams) == 0 {
		cfg.Hotkeys.SwapTeams = def.Hotkeys.SwapTeams
	}

	// Ensure sound volume is set if missing (0 means not set in config)
	if cfg.SoundVolume == 0 {
		cfg.SoundVolume = 1.0
	}

	// Ensure stats settings are set if missing
	if cfg.StatsPeriod == "" {
		cfg.StatsPeriod = "All Time"
	}
	if cfg.StatsGroup == "" {
		cfg.StatsGroup = "By Day"
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
