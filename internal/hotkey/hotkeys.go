//go:build linux || windows

package hotkey

import (
	"strings"
	"sync"
	"time"

	hook "github.com/robotn/gohook"
)

// ActionType represents the type of action triggered by a hotkey
type ActionType int

const (
	ActionNone ActionType = iota
	ActionIncrementCT
	ActionDecrementCT
	ActionIncrementT
	ActionDecrementT
	ActionReset
	ActionSelectCT
	ActionSelectT
	ActionSwapTeams
)

// Bindings holds the key combinations for each action
type Bindings struct {
	IncrementCT []string
	DecrementCT []string
	IncrementT  []string
	DecrementT  []string
	Reset       []string
	SelectCT    []string
	SelectT     []string
	SwapTeams   []string
}

// Handler processes keyboard events and triggers actions
type Handler struct {
	bindings       *Bindings
	pressedKeys    map[string]bool
	keysMutex      sync.Mutex
	lastActionTime time.Time
	hookChan       chan hook.Event
	hookRunning    bool
	actionChan     chan ActionType
}

// NewHandler creates a new hotkey handler
func NewHandler(bindings *Bindings) *Handler {
	return &Handler{
		bindings:    bindings,
		pressedKeys: make(map[string]bool),
		actionChan:  make(chan ActionType, 10),
	}
}

// Actions returns the channel for receiving triggered actions
func (h *Handler) Actions() <-chan ActionType {
	return h.actionChan
}

// UpdateBindings updates the hotkey bindings
func (h *Handler) UpdateBindings(bindings *Bindings) {
	h.keysMutex.Lock()
	defer h.keysMutex.Unlock()
	h.bindings = bindings
}

// Start begins listening for global keyboard events
func (h *Handler) Start() {
	if h.hookRunning {
		return
	}

	h.hookChan = hook.Start()
	h.hookRunning = true

	go func() {
		for ev := range h.hookChan {
			keyName := mapKeyToName(ev)
			if keyName == "" {
				continue
			}

			switch ev.Kind {
			case hook.KeyDown:
				h.handleKeyDown(keyName)
			case hook.KeyUp:
				h.handleKeyUp(keyName)
			}
		}
	}()
}

// Stop stops listening for keyboard events
func (h *Handler) Stop() {
	if h.hookRunning {
		hook.End()
		h.hookRunning = false
	}
}

func (h *Handler) handleKeyDown(keyName string) {
	h.keysMutex.Lock()
	defer h.keysMutex.Unlock()

	// Skip if key is already pressed (avoid repeat events)
	if h.pressedKeys[keyName] {
		return
	}

	h.pressedKeys[keyName] = true

	// Check for action cooldown (prevent rapid-fire)
	if time.Since(h.lastActionTime) < 100*time.Millisecond {
		return
	}

	// Check all hotkey combos
	var action ActionType
	if h.matchesCombo(h.bindings.IncrementCT) {
		action = ActionIncrementCT
	} else if h.matchesCombo(h.bindings.DecrementCT) {
		action = ActionDecrementCT
	} else if h.matchesCombo(h.bindings.IncrementT) {
		action = ActionIncrementT
	} else if h.matchesCombo(h.bindings.DecrementT) {
		action = ActionDecrementT
	} else if h.matchesCombo(h.bindings.Reset) {
		action = ActionReset
	} else if h.matchesCombo(h.bindings.SelectCT) {
		action = ActionSelectCT
	} else if h.matchesCombo(h.bindings.SelectT) {
		action = ActionSelectT
	} else if h.matchesCombo(h.bindings.SwapTeams) {
		action = ActionSwapTeams
	}

	if action != ActionNone {
		h.lastActionTime = time.Now()
		select {
		case h.actionChan <- action:
		default:
			// Channel full, skip action
		}
	}
}

func (h *Handler) handleKeyUp(keyName string) {
	h.keysMutex.Lock()
	defer h.keysMutex.Unlock()
	delete(h.pressedKeys, keyName)
}

func (h *Handler) matchesCombo(comboKeys []string) bool {
	// All keys in the combo must be pressed (case-insensitive for letters)
	for _, key := range comboKeys {
		found := false
		keyLower := strings.ToLower(key)
		for pressedKey := range h.pressedKeys {
			if strings.ToLower(pressedKey) == keyLower {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	// And we must have exactly the same number of keys pressed
	return len(h.pressedKeys) == len(comboKeys)
}

// mapKeyToName is defined in platform-specific files:
// - keymap_linux.go (X11 keysyms)
// - keymap_windows.go (Windows Virtual Key codes)
