package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"csstatstracker/internal/config"
)

// SettingsTab manages the settings view
type SettingsTab struct {
	cfg      *config.Config
	window   fyne.Window
	onSave   func(*config.Config)
	container fyne.CanvasObject
}

// NewSettingsTab creates a new settings tab
func NewSettingsTab(cfg *config.Config, window fyne.Window, onSave func(*config.Config)) *SettingsTab {
	s := &SettingsTab{
		cfg:    cfg,
		window: window,
		onSave: onSave,
	}
	s.container = s.buildUI()
	return s
}

// Container returns the tab content
func (s *SettingsTab) Container() fyne.CanvasObject {
	return s.container
}

func (s *SettingsTab) buildUI() fyne.CanvasObject {
	// Sound toggle
	soundCheck := widget.NewCheck("Enable Sound Effects", func(enabled bool) {
		s.cfg.SoundEnabled = enabled
		s.save()
	})
	soundCheck.Checked = s.cfg.SoundEnabled

	// Create buttons for each hotkey
	var incCTButton, decCTButton, incTButton, decTButton, resetHotkeyButton, selectCTButton, selectTButton *widget.Button

	incCTButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.IncrementCT), func() {
		CaptureHotkey(s.window, "Increment CT", &s.cfg.Hotkeys.IncrementCT, incCTButton, s.save)
	})

	decCTButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.DecrementCT), func() {
		CaptureHotkey(s.window, "Decrement CT", &s.cfg.Hotkeys.DecrementCT, decCTButton, s.save)
	})

	incTButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.IncrementT), func() {
		CaptureHotkey(s.window, "Increment T", &s.cfg.Hotkeys.IncrementT, incTButton, s.save)
	})

	decTButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.DecrementT), func() {
		CaptureHotkey(s.window, "Decrement T", &s.cfg.Hotkeys.DecrementT, decTButton, s.save)
	})

	resetHotkeyButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.Reset), func() {
		CaptureHotkey(s.window, "Reset", &s.cfg.Hotkeys.Reset, resetHotkeyButton, s.save)
	})

	selectCTButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.SelectCT), func() {
		CaptureHotkey(s.window, "Select CT Team", &s.cfg.Hotkeys.SelectCT, selectCTButton, s.save)
	})

	selectTButton = widget.NewButton(FormatHotkeys(s.cfg.Hotkeys.SelectT), func() {
		CaptureHotkey(s.window, "Select T Team", &s.cfg.Hotkeys.SelectT, selectTButton, s.save)
	})

	form := container.NewVBox(
		soundCheck,
		widget.NewLabel("Hotkey Configuration (click to change)"),
		widget.NewForm(
			widget.NewFormItem("Increment CT", incCTButton),
			widget.NewFormItem("Decrement CT", decCTButton),
			widget.NewFormItem("Increment T", incTButton),
			widget.NewFormItem("Decrement T", decTButton),
			widget.NewFormItem("Reset", resetHotkeyButton),
			widget.NewFormItem("Select CT Team", selectCTButton),
			widget.NewFormItem("Select T Team", selectTButton),
		),
	)

	return form
}

func (s *SettingsTab) save() {
	if s.onSave != nil {
		s.onSave(s.cfg)
	}
}

// FormatHotkeys formats a slice of key names as a display string
func FormatHotkeys(keys []string) string {
	if len(keys) == 0 {
		return "Not set"
	}
	result := ""
	for i, key := range keys {
		if i > 0 {
			result += "+"
		}
		result += key
	}
	return result
}

// CaptureHotkey opens a dialog to capture a key combination
func CaptureHotkey(w fyne.Window, action string, target *[]string, button *widget.Button, onSave func()) {
	tempWindow := fyne.CurrentApp().NewWindow("Key Capture")
	tempWindow.Resize(fyne.Size{Width: 400, Height: 200})
	tempWindow.CenterOnScreen()

	label := widget.NewLabel(fmt.Sprintf("Press key combination for: %s", action))
	label.Alignment = fyne.TextAlignCenter
	label.Wrapping = fyne.TextWrapWord

	statusLabel := widget.NewLabel("Waiting for keys...")
	statusLabel.Alignment = fyne.TextAlignCenter

	var capturedCombo []string
	var captureMutex sync.Mutex

	okButton := widget.NewButton("OK", func() {
		captureMutex.Lock()
		defer captureMutex.Unlock()

		if len(capturedCombo) > 0 {
			*target = capturedCombo
			button.SetText(FormatHotkeys(capturedCombo))
			if onSave != nil {
				onSave()
			}
		}
		tempWindow.Close()
	})
	okButton.Importance = widget.HighImportance
	okButton.Disable()

	clearButton := widget.NewButton("Clear", func() {
		captureMutex.Lock()
		capturedCombo = []string{}
		captureMutex.Unlock()
		statusLabel.SetText("Cleared. Press keys...")
		okButton.Disable()
	})

	cancelButton := widget.NewButton("Cancel", func() {
		tempWindow.Close()
	})

	buttons := container.NewHBox(
		layout.NewSpacer(),
		okButton,
		clearButton,
		cancelButton,
		layout.NewSpacer(),
	)

	content := container.NewVBox(
		layout.NewSpacer(),
		label,
		statusLabel,
		widget.NewLabel("Press your key combination, then click OK"),
		buttons,
		layout.NewSpacer(),
	)

	// Track currently held keys
	heldKeys := make(map[string]bool)

	if deskCanvas, ok := tempWindow.Canvas().(desktop.Canvas); ok {
		deskCanvas.SetOnKeyDown(func(key *fyne.KeyEvent) {
			captureMutex.Lock()
			keyStr := string(key.Name)

			// Track this key as held
			heldKeys[keyStr] = true

			// Add to combo if not already there
			if !containsKey(capturedCombo, keyStr) {
				capturedCombo = append(capturedCombo, keyStr)
			}

			captureMutex.Unlock()
			statusLabel.SetText(fmt.Sprintf("Keys: %v", capturedCombo))
			okButton.Enable()
		})

		deskCanvas.SetOnKeyUp(func(key *fyne.KeyEvent) {
			captureMutex.Lock()
			keyStr := string(key.Name)
			delete(heldKeys, keyStr)
			captureMutex.Unlock()
		})
	} else {
		// Fallback for non-desktop canvas
		tempWindow.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
			captureMutex.Lock()
			keyStr := string(key.Name)

			if !containsKey(capturedCombo, keyStr) {
				capturedCombo = append(capturedCombo, keyStr)
			}

			captureMutex.Unlock()
			statusLabel.SetText(fmt.Sprintf("Keys: %v", capturedCombo))
			okButton.Enable()
		})
	}

	tempWindow.SetContent(content)
	tempWindow.Show()
}

// containsKey checks if a key is already in the slice
func containsKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}
