package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// AutoSizeEntry is an entry widget that sizes itself based on content
type AutoSizeEntry struct {
	widget.Entry
}

// NewAutoSizeEntry creates a new auto-sizing entry widget
func NewAutoSizeEntry() *AutoSizeEntry {
	e := &AutoSizeEntry{}
	e.ExtendBaseWidget(e)
	return e
}

// MinSize returns the minimum size based on text content
func (e *AutoSizeEntry) MinSize() fyne.Size {
	textLen := len(e.Text)
	if textLen < 1 {
		textLen = 1
	}
	width := float32(textLen*10 + 16) // 10px per char + 16px padding
	if width < 26 {
		width = 26 // minimum width
	}
	return fyne.NewSize(width, e.Entry.MinSize().Height)
}
