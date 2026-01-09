package ui

import (
	"context"
	"database/sql"
	"fmt"
	"image/color"
	"strconv"

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
	selectedColor   = color.RGBA{R: 70, G: 130, B: 180, A: 255}  // Steel blue
	unselectedColor = color.RGBA{R: 0, G: 0, B: 0, A: 0}         // Transparent
)

// selectableRow is a tappable row that supports selection with highlighting
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
		// Shift-click: select range
		r.history.selectRange(r.rowIdx)
	} else {
		// Normal click: select only this item (clear others)
		r.history.selectSingle(r.rowIdx)
	}
}

func (r *selectableRow) MouseIn(e *desktop.MouseEvent) {
	if r.history == nil {
		return
	}
	// Check if shift is held and mouse button is pressed (dragging)
	if e.Button == desktop.MouseButtonPrimary {
		if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
			if mod := d.CurrentKeyModifiers(); mod&fyne.KeyModifierShift != 0 {
				// Shift+drag: select this row
				if r.rowIdx >= 0 && r.rowIdx < len(r.history.games) {
					r.history.selected[r.history.games[r.rowIdx].ID] = true
					r.history.updateToolbar()
					r.history.list.Refresh()
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

// HistoryTab manages the game history view with CRUD operations
type HistoryTab struct {
	db             *sql.DB
	window         fyne.Window
	list           *widget.List
	games          []database.GameStats
	selected       map[int]bool
	lastClickedIdx int
	onUpdate       func()
	deleteBtn      *widget.Button
	selectAllBtn   *widget.Button
	clearBtn       *widget.Button
}

// NewHistoryTab creates a new history tab
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

// Container returns the tab content
func (h *HistoryTab) Container() fyne.CanvasObject {
	h.list = widget.NewList(
		func() int {
			return len(h.games)
		},
		func() fyne.CanvasObject {
			return newSelectableRow(h)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(h.games) {
				return
			}
			game := h.games[id]
			gameID := game.ID
			rowIdx := int(id)

			row := obj.(*selectableRow)
			row.rowIdx = rowIdx
			row.history = h

			dateStr := game.CreatedAt.Format("2006-01-02 15:04")
			winner := "Draw"
			if game.CTScore > game.TScore {
				winner = "CT"
			} else if game.TScore > game.CTScore {
				winner = "T"
			}
			teamStr := "None"
			if game.Team != "" {
				teamStr = string(game.Team)
			}
			row.label.SetText(fmt.Sprintf("%s | CT:%d T:%d (max:%d) %s [%s]", dateStr, game.CTScore, game.TScore, game.GameScore, winner, teamStr))

			// Update selection highlight
			row.SetSelected(h.selected[gameID])

			// Enable/disable edit based on selection count
			selectedCount := len(h.selected)
			if selectedCount > 1 {
				row.editBtn.Disable()
			} else {
				row.editBtn.Enable()
			}

			row.editBtn.OnTapped = func() {
				if len(h.selected) <= 1 {
					h.showEditDialog(&game)
				}
			}
			row.delBtn.OnTapped = func() {
				h.confirmDelete(&game)
			}
		},
	)

	// Disable separators and list's own selection
	h.list.HideSeparators = true
	h.list.OnSelected = func(id widget.ListItemID) {
		// Deselect in list widget (we manage our own selection)
		h.list.UnselectAll()
	}

	addBtn := widget.NewButton("+ Add Game", func() {
		h.showAddDialog()
	})
	addBtn.Importance = widget.HighImportance

	h.deleteBtn = widget.NewButton("Delete Selected", func() {
		h.confirmDeleteSelected()
	})
	h.deleteBtn.Importance = widget.DangerImportance
	h.deleteBtn.Hide()

	h.selectAllBtn = widget.NewButton("Select All", func() {
		for _, game := range h.games {
			h.selected[game.ID] = true
		}
		if len(h.games) > 0 {
			h.lastClickedIdx = 0
		}
		h.updateToolbar()
		h.list.Refresh()
	})

	h.clearBtn = widget.NewButton("Clear Selection", func() {
		h.selected = make(map[int]bool)
		h.lastClickedIdx = -1
		h.updateToolbar()
		h.list.Refresh()
	})
	h.clearBtn.Hide()

	refreshBtn := widget.NewButton("Refresh", func() {
		h.refresh()
	})

	toolbar := container.NewHBox(addBtn, h.deleteBtn, h.selectAllBtn, h.clearBtn, refreshBtn)

	return container.NewBorder(toolbar, nil, nil, nil, h.list)
}

// selectSingle clears selection and selects only the clicked item
func (h *HistoryTab) selectSingle(idx int) {
	if idx < 0 || idx >= len(h.games) {
		return
	}
	gameID := h.games[idx].ID

	// If clicking on the only selected item, deselect it
	if len(h.selected) == 1 && h.selected[gameID] {
		h.selected = make(map[int]bool)
	} else {
		// Clear all and select only this one
		h.selected = make(map[int]bool)
		h.selected[gameID] = true
	}
	h.lastClickedIdx = idx
	h.updateToolbar()
	h.list.Refresh()
}

// selectRange selects all items between lastClickedIdx and the current index
func (h *HistoryTab) selectRange(toIdx int) {
	if h.lastClickedIdx < 0 || h.lastClickedIdx >= len(h.games) {
		// No previous selection, just select this one
		if toIdx >= 0 && toIdx < len(h.games) {
			h.selected[h.games[toIdx].ID] = true
			h.lastClickedIdx = toIdx
		}
	} else {
		// Select range
		start, end := h.lastClickedIdx, toIdx
		if start > end {
			start, end = end, start
		}
		for i := start; i <= end && i < len(h.games); i++ {
			h.selected[h.games[i].ID] = true
		}
	}
	h.updateToolbar()
	h.list.Refresh()
}

func (h *HistoryTab) updateToolbar() {
	if h.deleteBtn == nil || h.clearBtn == nil {
		return
	}
	count := len(h.selected)
	if count > 1 {
		// Only show batch delete buttons when multiple items selected
		h.deleteBtn.SetText(fmt.Sprintf("Delete Selected (%d)", count))
		h.deleteBtn.Show()
		h.clearBtn.Show()
	} else {
		h.deleteBtn.Hide()
		h.clearBtn.Hide()
	}
}

// Refresh reloads data from database
func (h *HistoryTab) Refresh() {
	h.refresh()
}

func (h *HistoryTab) refresh() {
	ctx := context.Background()
	games, err := database.GetAllGames(ctx, h.db)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}
	h.games = games
	h.selected = make(map[int]bool)
	h.lastClickedIdx = -1
	h.updateToolbar()
	if h.list != nil {
		h.list.Refresh()
	}
}

func (h *HistoryTab) showAddDialog() {
	ctEntry := widget.NewEntry()
	ctEntry.SetPlaceHolder("0")
	tEntry := widget.NewEntry()
	tEntry.SetPlaceHolder("0")
	maxEntry := widget.NewEntry()
	maxEntry.SetPlaceHolder("8")
	teamSelect := widget.NewSelect([]string{"None", "CT", "T"}, nil)
	teamSelect.SetSelected("None")

	form := widget.NewForm(
		widget.NewFormItem("CT Score", ctEntry),
		widget.NewFormItem("T Score", tEntry),
		widget.NewFormItem("Game Score", maxEntry),
		widget.NewFormItem("Player's Team", teamSelect),
	)

	dialog.ShowCustomConfirm("Add Game", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		ct, _ := strconv.Atoi(ctEntry.Text)
		t, _ := strconv.Atoi(tEntry.Text)
		max, _ := strconv.Atoi(maxEntry.Text)
		if max <= 0 {
			max = 8
		}
		team := database.TeamNone
		if teamSelect.Selected != "None" {
			team = database.Team(teamSelect.Selected)
		}

		ctx := context.Background()
		if err := database.SaveGame(ctx, h.db, ct, t, max, team); err != nil {
			dialog.ShowError(err, h.window)
			return
		}
		h.refresh()
		if h.onUpdate != nil {
			h.onUpdate()
		}
	}, h.window)
}

func (h *HistoryTab) showEditDialog(game *database.GameStats) {
	ctEntry := widget.NewEntry()
	ctEntry.SetText(strconv.Itoa(game.CTScore))
	tEntry := widget.NewEntry()
	tEntry.SetText(strconv.Itoa(game.TScore))
	maxEntry := widget.NewEntry()
	maxEntry.SetText(strconv.Itoa(game.GameScore))
	teamSelect := widget.NewSelect([]string{"None", "CT", "T"}, nil)
	if game.Team == "" {
		teamSelect.SetSelected("None")
	} else {
		teamSelect.SetSelected(string(game.Team))
	}

	form := widget.NewForm(
		widget.NewFormItem("CT Score", ctEntry),
		widget.NewFormItem("T Score", tEntry),
		widget.NewFormItem("Game Score", maxEntry),
		widget.NewFormItem("Player's Team", teamSelect),
	)

	dialog.ShowCustomConfirm("Edit Game", "Save", "Cancel", form, func(save bool) {
		if !save {
			return
		}
		ct, _ := strconv.Atoi(ctEntry.Text)
		t, _ := strconv.Atoi(tEntry.Text)
		max, _ := strconv.Atoi(maxEntry.Text)
		if max <= 0 {
			max = 8
		}
		team := database.TeamNone
		if teamSelect.Selected != "None" {
			team = database.Team(teamSelect.Selected)
		}

		ctx := context.Background()
		if err := database.UpdateGame(ctx, h.db, game.ID, ct, t, max, team); err != nil {
			dialog.ShowError(err, h.window)
			return
		}
		h.refresh()
		if h.onUpdate != nil {
			h.onUpdate()
		}
	}, h.window)
}

func (h *HistoryTab) confirmDelete(game *database.GameStats) {
	dialog.ShowConfirm("Delete Game",
		fmt.Sprintf("Delete game from %s?", game.CreatedAt.Format("2006-01-02 15:04")),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			ctx := context.Background()
			if err := database.DeleteGame(ctx, h.db, game.ID); err != nil {
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

	dialog.ShowConfirm("Delete Games",
		fmt.Sprintf("Delete %d selected game(s)?", count),
		func(confirmed bool) {
			if !confirmed {
				return
			}
			ctx := context.Background()
			for id := range h.selected {
				if err := database.DeleteGame(ctx, h.db, id); err != nil {
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
