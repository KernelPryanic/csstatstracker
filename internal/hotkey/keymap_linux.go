//go:build linux

package hotkey

import hook "github.com/robotn/gohook"

// mapKeyToName converts a gohook event to a key name string (Linux/X11 version)
func mapKeyToName(ev hook.Event) string {
	// Map based on rawcode (X11 keysyms)
	switch ev.Rawcode {
	// Modifier keys (X11 keysyms)
	case 65505:
		return "LeftShift"
	case 65506:
		return "RightShift"
	case 65507:
		return "LeftControl"
	case 65508:
		return "RightControl"
	case 65513:
		return "LeftAlt"
	case 65514:
		return "RightAlt"
	case 65515:
		return "LeftSuper"
	case 65516:
		return "RightSuper"

	// Function keys (X11 keysyms)
	case 65470:
		return "F1"
	case 65471:
		return "F2"
	case 65472:
		return "F3"
	case 65473:
		return "F4"
	case 65474:
		return "F5"
	case 65475:
		return "F6"
	case 65476:
		return "F7"
	case 65477:
		return "F8"
	case 65478:
		return "F9"
	case 65479:
		return "F10"
	case 65480:
		return "F11"
	case 65481:
		return "F12"

	// Special keys
	case 65293:
		return "Return"
	case 65288:
		return "Backspace"
	case 65289:
		return "Tab"
	case 32:
		return "Space"
	case 65307:
		return "Escape"

	// Numpad keys (X11 keysyms)
	case 65457:
		return "Numpad1"
	case 65458:
		return "Numpad2"
	case 65459:
		return "Numpad3"
	case 65460:
		return "Numpad4"
	case 65461:
		return "Numpad5"
	case 65462:
		return "Numpad6"
	case 65463:
		return "Numpad7"
	case 65464:
		return "Numpad8"
	case 65465:
		return "Numpad9"
	case 65456:
		return "Numpad0"
	case 65454:
		return "NumpadDecimal"
	case 65451:
		return "NumpadAdd"
	case 65453:
		return "NumpadSubtract"
	case 65450:
		return "NumpadMultiply"
	case 65455:
		return "NumpadDivide"
	case 65421:
		return "NumpadEnter"

	// Common symbol keys by their rawcode (unshifted X11 keysyms)
	case 45: // minus key
		return "-"
	case 61: // equals key
		return "="
	case 43: // plus (shifted equals)
		return "="
	case 95: // underscore (shifted minus)
		return "-"

	// Letter keys by rawcode (needed for Ctrl+letter combos where keychar becomes control char)
	case 97:
		return "a"
	case 98:
		return "b"
	case 99:
		return "c"
	case 100:
		return "d"
	case 101:
		return "e"
	case 102:
		return "f"
	case 103:
		return "g"
	case 104:
		return "h"
	case 105:
		return "i"
	case 106:
		return "j"
	case 107:
		return "k"
	case 108:
		return "l"
	case 109:
		return "m"
	case 110:
		return "n"
	case 111:
		return "o"
	case 112:
		return "p"
	case 113:
		return "q"
	case 114:
		return "r"
	case 115:
		return "s"
	case 116:
		return "t"
	case 117:
		return "u"
	case 118:
		return "v"
	case 119:
		return "w"
	case 120:
		return "x"
	case 121:
		return "y"
	case 122:
		return "z"
	}

	// For printable characters, use the keychar directly
	if ev.Keychar >= 32 && ev.Keychar <= 126 {
		return string(rune(ev.Keychar))
	}

	// Return empty if we can't map it
	return ""
}
