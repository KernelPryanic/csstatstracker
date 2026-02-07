//go:build windows

package config

// defaultHotkeys returns the default hotkey bindings for Windows
// Windows uses character equivalents for numpad keys to match Fyne's capture
func defaultHotkeys() Hotkeys {
	return Hotkeys{
		IncrementCT: []string{"1", "+"},
		DecrementCT: []string{"1", "-"},
		IncrementT:  []string{"2", "+"},
		DecrementT:  []string{"2", "-"},
		Reset:       []string{"0", "Return"},
		SelectCT:    []string{"LeftControl", "C"},
		SelectT:     []string{"LeftControl", "T"},
		SwapTeams:   []string{"LeftControl", "S"},
	}
}
