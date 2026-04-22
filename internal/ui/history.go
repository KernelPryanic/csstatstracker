package ui

import (
	"context"
	"database/sql"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"csstatstracker/internal/database"
)

var (
	selectedColor   = color.RGBA{R: 70, G: 130, B: 180, A: 255}
	unselectedColor = color.RGBA{R: 0, G: 0, B: 0, A: 0}
)

// selectableRow is a tappable row that supports selection highlighting.
type selectableRow struct {
	widget.BaseWidget
	background *canvas.Rectangle
	label      *widget.Label
	editBtn    *widget.Button
	delBtn     *widget.Button
	content    *fyne.Container

	rowIdx     int
	isSelected bool
	history    *HistoryTab
}

func newSelectableRow(h *HistoryTab) *selectableRow {
	r := &selectableRow{
		history:    h,
		background: canvas.NewRectangle(unselectedColor),
		label:      widget.NewLabel("template"),
		editBtn:    widget.NewButton("Edit", nil),
		delBtn:     widget.NewButton("Delete", nil),
	}
	r.ExtendBaseWidget(r)

	row := container.NewHBox(
		r.label,
		layout.NewSpacer(),
		r.editBtn,
		r.delBtn,
	)
	r.content = container.NewStack(r.background, row)
	return r
}

func (r *selectableRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

func (r *selectableRow) Tapped(e *fyne.PointEvent) {
	if r.history == nil {
		return
	}
	shiftHeld := false
	if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
		if mod := d.CurrentKeyModifiers(); mod&fyne.KeyModifierShift != 0 {
			shiftHeld = true
		}
	}
	if shiftHeld && r.history.lastClickedIdx >= 0 {
		r.history.selectRange(r.rowIdx)
	} else {
		r.history.selectSingle(r.rowIdx)
	}
}

func (r *selectableRow) MouseIn(e *desktop.MouseEvent) {
	if r.history == nil {
		return
	}
	if e.Button == desktop.MouseButtonPrimary {
		if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
			if mod := d.CurrentKeyModifiers(); mod&fyne.KeyModifierShift != 0 {
				if r.rowIdx >= 0 && r.rowIdx < len(r.history.rounds) {
					r.history.selected[r.history.rounds[r.rowIdx].ID] = true
					r.history.updateToolbar()
					r.history.refreshRows()
				}
			}
		}
	}
}

func (r *selectableRow) MouseMoved(e *desktop.MouseEvent) {}
func (r *selectableRow) MouseOut()                        {}

func (r *selectableRow) SetSelected(selected bool) {
	r.isSelected = selected
	if selected {
		r.background.FillColor = selectedColor
	} else {
		r.background.FillColor = unselectedColor
	}
	r.background.Refresh()
}

// HistoryTab shows every recorded round with edit / delete controls.
type HistoryTab struct {
	db             *sql.DB
	window         fyne.Window
	list           *widget.List
	rounds         []database.Round
	selected       map[int]bool
	lastClickedIdx int
	onUpdate       func()
	deleteBtn      *widget.Button
	selectAllBtn   *widget.Button
	clearBtn       *widget.Button
}

// NewHistoryTab creates a new history tab.
func NewHistoryTab(db *sql.DB, window fyne.Window, onUpdate func()) *HistoryTab {
	h := &HistoryTab{
		db:             db,
		window:         window,
		onUpdate:       onUpdate,
		selected:       make(map[int]bool),
		lastClickedIdx: -1,
	}
	h.refresh()
	return h
}

// Container returns the tab content.
func (h *HistoryTab) Container() fyne.CanvasObject {
	// widget.List virtualises — only visible rows are materialised, which is
	// essential when a user has thousands of rounds in history.
	h.list = widget.NewList(
		func() int { return len(h.rounds) },
		func() fyne.CanvasObject { return newSelectableRow(h) },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(h.rounds) {
				return
			}
			r := h.rounds[id]
			row := obj.(*selectableRow)
			row.rowIdx = id
			row.history = h

			teamStr := "None"
			if r.Team != "" {
				teamStr = string(r.Team)
			}
			row.label.SetText(fmt.Sprintf("%s | %s won [%s]",
				r.CreatedAt.Format("2006-01-02 15:04:05"),
				r.Winner,
				teamStr,
			))
			row.SetSelected(h.selected[r.ID])

			if len(h.selected) > 1 {
				row.editBtn.Disable()
			} else {
				row.editBtn.Enable()
			}

			rnd := r
			row.editBtn.OnTapped = func() {
				if len(h.selected) <= 1 {
					h.showEditDialog(&rnd)
				}
			}
			row.delBtn.OnTapped = func() { h.confirmDelete(&rnd) }
		},
	)
	h.list.HideSeparators = true
	h.list.OnSelected = func(id widget.ListItemID) { h.list.UnselectAll() }

	addBtn := widget.NewButton("+ Add Round", func() {
		h.showAddDialog()
	})
	addBtn.Importance = widget.HighImportance

	h.deleteBtn = widget.NewButton("Delete Selected", func() {
		h.confirmDeleteSelected()
	})
	h.deleteBtn.Importance = widget.DangerImportance
	h.deleteBtn.Hide()

	h.selectAllBtn = widget.NewButton("Select All", func() {
		for _, r := range h.rounds {
			h.selected[r.ID] = true
		}
		if len(h.rounds) > 0 {
			h.lastClickedIdx = 0
		}
		h.updateToolbar()
		h.refreshRows()
	})

	h.clearBtn = widget.NewButton("Clear Selection", func() {
		h.selected = make(map[int]bool)
		h.lastClickedIdx = -1
		h.updateToolbar()
		h.refreshRows()
	})
	h.clearBtn.Hide()

	refreshBtn := widget.NewButton("Refresh", func() {
		h.refresh()
	})

	toolbar := container.NewHBox(addBtn, h.deleteBtn, h.selectAllBtn, h.clearBtn, refreshBtn)
	return container.NewBorder(toolbar, nil, nil, nil, h.list)
}

// refreshRows redraws the currently-visible list rows.
func (h *HistoryTab) refreshRows() {
	if h.list != nil {
		h.list.Refresh()
	}
}

func (h *HistoryTab) selectSingle(idx int) {
	if idx < 0 || idx >= len(h.rounds) {
		return
	}
	id := h.rounds[idx].ID
	if len(h.selected) == 1 && h.selected[id] {
		h.selected = make(map[int]bool)
	} else {
		h.selected = make(map[int]bool)
		h.selected[id] = true
	}
	h.lastClickedIdx = idx
	h.updateToolbar()
	h.refreshRows()
}

func (h *HistoryTab) selectRange(toIdx int) {
	if h.lastClickedIdx < 0 || h.lastClickedIdx >= len(h.rounds) {
		if toIdx >= 0 && toIdx < len(h.rounds) {
			h.selected[h.rounds[toIdx].ID] = true
			h.lastClickedIdx = toIdx
		}
	} else {
		start, end := h.lastClickedIdx, toIdx
		if start > end {
			start, end = end, start
		}
		for i := start; i <= end && i < len(h.rounds); i++ {
			h.selected[h.rounds[i].ID] = true
		}
	}
	h.updateToolbar()
	h.refreshRows()
}

func (h *HistoryTab) updateToolbar() {
	if h.deleteBtn == nil || h.clearBtn == nil {
		return
	}
	count := len(h.selected)
	if count > 1 {
		h.deleteBtn.SetText(fmt.Sprintf("Delete Selected (%d)", count))
		h.deleteBtn.Show()
		h.clearBtn.Show()
	} else {
		h.deleteBtn.Hide()
		h.clearBtn.Hide()
	}
}

// Refresh reloads data from database.
func (h *HistoryTab) Refresh() { h.refresh() }

func (h *HistoryTab) refresh() {
	ctx := context.Background()
	rounds, err := database.GetAllRounds(ctx, h.db)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}
	h.rounds = rounds
	h.selected = make(map[int]bool)
	h.lastClickedIdx = -1
	h.updateToolbar()
	h.refreshRows()
}

func (h *HistoryTab) showAddDialog() {
	winnerSelect := widget.NewSelect([]string{"CT", "T"}, nil)
	winnerSelect.SetSelected("CT")
	teamSelect := widget.NewSelect([]string{"None", "CT", "T"}, nil)
	teamSelect.SetSelected("None")

	form := widget.NewForm(
		widget.NewFormItem("Winner", winnerSelect),
		widget.NewFormItem("Your Team", teamSelect),
	)

	dialog.ShowCustomConfirm("Add Round", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		winner := database.Team(winnerSelect.Selected)
		team := database.TeamNone
		if teamSelect.Selected != "None" {
			team = database.Team(teamSelect.Selected)
		}
		if _, err := database.InsertRound(context.Background(), h.db, winner, team); err != nil {
			dialog.ShowError(err, h.window)
			return
		}
		h.refresh()
		if h.onUpdate != nil {
			h.onUpdate()
		}
	}, h.window)
}

func (h *HistoryTab) showEditDialog(r *database.Round) {
	winnerSelect := widget.NewSelect([]string{"CT", "T"}, nil)
	winnerSelect.SetSelected(string(r.Winner))
	teamSelect := widget.NewSelect([]string{"None", "CT", "T"}, nil)
	if r.Team == "" {
		teamSelect.SetSelected("None")
	} else {
		teamSelect.SetSelected(string(r.Team))
	}
	tsLabel := widget.NewLabel(r.CreatedAt.Format("2006-01-02 15:04:05"))

	form := widget.NewForm(
		widget.NewFormItem("Timestamp", tsLabel),
		widget.NewFormItem("Winner", winnerSelect),
		widget.NewFormItem("Your Team", teamSelect),
	)

	dialog.ShowCustomConfirm("Edit Round", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		winner := database.Team(winnerSelect.Selected)
		team := database.TeamNone
		if teamSelect.Selected != "None" {
			team = database.Team(teamSelect.Selected)
		}
		if err := database.UpdateRound(context.Background(), h.db, r.ID, winner, team); err != nil {
			dialog.ShowError(err, h.window)
			return
		}
		h.refresh()
		if h.onUpdate != nil {
			h.onUpdate()
		}
	}, h.window)
}

func (h *HistoryTab) confirmDelete(r *database.Round) {
	dialog.ShowConfirm("Delete Round",
		fmt.Sprintf("Delete round from %s?", r.CreatedAt.Format("2006-01-02 15:04:05")),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			if err := database.DeleteRound(context.Background(), h.db, r.ID); err != nil {
				dialog.ShowError(err, h.window)
				return
			}
			h.refresh()
			if h.onUpdate != nil {
				h.onUpdate()
			}
		}, h.window)
}

func (h *HistoryTab) confirmDeleteSelected() {
	count := len(h.selected)
	if count == 0 {
		return
	}
	dialog.ShowConfirm("Delete Rounds",
		fmt.Sprintf("Delete %d selected round(s)?", count),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			ctx := context.Background()
			for id := range h.selected {
				if err := database.DeleteRound(ctx, h.db, id); err != nil {
					dialog.ShowError(err, h.window)
					return
				}
			}
			h.refresh()
			if h.onUpdate != nil {
				h.onUpdate()
			}
		}, h.window)
}
