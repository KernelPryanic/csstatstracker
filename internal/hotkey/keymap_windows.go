//go:build windows

package hotkey

import (
	"strings"

	hook "github.com/robotn/gohook"
)

// mapKeyToName converts a gohook event to a key name string (Windows version)
// Windows uses Virtual Key codes in rawcode
func mapKeyToName(ev hook.Event) string {
	// First try to map based on rawcode (Windows Virtual Key codes)
	if name := mapRawcode(ev.Rawcode); name != "" {
		return name
	}

	// Fallback: use keychar for printable characters
	if ev.Keychar >= 32 && ev.Keychar <= 126 {
		// Return uppercase for letters to match Fyne key names
		return strings.ToUpper(string(rune(ev.Keychar)))
	}

	return ""
}

func mapRawcode(rawcode uint16) string {
	switch rawcode {
	// Modifier keys (Windows VK codes)
	case 160: // VK_LSHIFT
		return "LeftShift"
	case 161: // VK_RSHIFT
		return "RightShift"
	case 162: // VK_LCONTROL
		return "LeftControl"
	case 163: // VK_RCONTROL
		return "RightControl"
	case 164: // VK_LMENU (Left Alt)
		return "LeftAlt"
	case 165: // VK_RMENU (Right Alt)
		return "RightAlt"
	case 91: // VK_LWIN
		return "LeftSuper"
	case 92: // VK_RWIN
		return "RightSuper"

	// Function keys (Windows VK codes)
	case 112: // VK_F1
		return "F1"
	case 113: // VK_F2
		return "F2"
	case 114: // VK_F3
		return "F3"
	case 115: // VK_F4
		return "F4"
	case 116: // VK_F5
		return "F5"
	case 117: // VK_F6
		return "F6"
	case 118: // VK_F7
		return "F7"
	case 119: // VK_F8
		return "F8"
	case 120: // VK_F9
		return "F9"
	case 121: // VK_F10
		return "F10"
	case 122: // VK_F11
		return "F11"
	case 123: // VK_F12
		return "F12"

	// Special keys
	case 13: // VK_RETURN
		return "Return"
	case 8: // VK_BACK
		return "Backspace"
	case 9: // VK_TAB
		return "Tab"
	case 32: // VK_SPACE
		return "Space"
	case 27: // VK_ESCAPE
		return "Escape"

	// Numpad keys (Windows VK codes)
	// Fyne on Windows reports numpad keys as their character equivalents
	case 97: // VK_NUMPAD1
		return "1"
	case 98: // VK_NUMPAD2
		return "2"
	case 99: // VK_NUMPAD3
		return "3"
	case 100: // VK_NUMPAD4
		return "4"
	case 101: // VK_NUMPAD5
		return "5"
	case 102: // VK_NUMPAD6
		return "6"
	case 103: // VK_NUMPAD7
		return "7"
	case 104: // VK_NUMPAD8
		return "8"
	case 105: // VK_NUMPAD9
		return "9"
	case 96: // VK_NUMPAD0
		return "0"
	case 110: // VK_DECIMAL
		return "."
	case 107: // VK_ADD
		return "+"
	case 109: // VK_SUBTRACT
		return "-"
	case 106: // VK_MULTIPLY
		return "*"
	case 111: // VK_DIVIDE
		return "/"
	// NumpadEnter shares VK_RETURN (13)

	// Symbol keys
	case 189: // VK_OEM_MINUS
		return "-"
	case 187: // VK_OEM_PLUS (equals/plus key)
		return "="

	// Letter keys (Windows VK codes are uppercase ASCII)
	// Return uppercase to match Fyne's key names
	case 65: // VK_A
		return "A"
	case 66: // VK_B
		return "B"
	case 67: // VK_C
		return "C"
	case 68: // VK_D
		return "D"
	case 69: // VK_E
		return "E"
	case 70: // VK_F
		return "F"
	case 71: // VK_G
		return "G"
	case 72: // VK_H
		return "H"
	case 73: // VK_I
		return "I"
	case 74: // VK_J
		return "J"
	case 75: // VK_K
		return "K"
	case 76: // VK_L
		return "L"
	case 77: // VK_M
		return "M"
	case 78: // VK_N
		return "N"
	case 79: // VK_O
		return "O"
	case 80: // VK_P
		return "P"
	case 81: // VK_Q
		return "Q"
	case 82: // VK_R
		return "R"
	case 83: // VK_S
		return "S"
	case 84: // VK_T
		return "T"
	case 85: // VK_U
		return "U"
	case 86: // VK_V
		return "V"
	case 87: // VK_W
		return "W"
	case 88: // VK_X
		return "X"
	case 89: // VK_Y
		return "Y"
	case 90: // VK_Z
		return "Z"

	// Number keys (top row)
	case 48: // VK_0
		return "0"
	case 49: // VK_1
		return "1"
	case 50: // VK_2
		return "2"
	case 51: // VK_3
		return "3"
	case 52: // VK_4
		return "4"
	case 53: // VK_5
		return "5"
	case 54: // VK_6
		return "6"
	case 55: // VK_7
		return "7"
	case 56: // VK_8
		return "8"
	case 57: // VK_9
		return "9"
	}

	return ""
}
