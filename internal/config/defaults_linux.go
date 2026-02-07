//go:build linux

package config

// defaultHotkeys returns the default hotkey bindings for Linux
func defaultHotkeys() Hotkeys {
	return Hotkeys{
		IncrementCT: []string{"Numpad1", "NumpadAdd"},
		DecrementCT: []string{"Numpad1", "NumpadSubtract"},
		IncrementT:  []string{"Numpad2", "NumpadAdd"},
		DecrementT:  []string{"Numpad2", "NumpadSubtract"},
		Reset:       []string{"Numpad0", "NumpadEnter"},
		SelectCT:    []string{"LeftControl", "c"},
		SelectT:     []string{"LeftControl", "t"},
		SwapTeams:   []string{"LeftControl", "s"},
	}
}
