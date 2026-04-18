package ui

import (
	"context"
	"database/sql"
	"fmt"
	"image/color"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"csstatstracker/internal/database"
)

var (
	selectedColor   = color.RGBA{R: 70, G: 130, B: 180, A: 255} // Steel blue
	unselectedColor = color.RGBA{R: 0, G: 0, B: 0, A: 0}        // Transparent
)

// selectableRow is a tappable row that supports selection with highlighting
// and an inline-expandable rounds panel beneath the header.
type selectableRow struct {
	widget.BaseWidget
	background *canvas.Rectangle
	label      *widget.Label
	editBtn    *widget.Button
	delBtn     *widget.Button
	expandBtn  *widget.Button
	roundsBox  *fyne.Container
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
		expandBtn:  widget.NewButton("▸", nil),
		roundsBox:  container.NewVBox(),
	}
	r.expandBtn.Importance = widget.LowImportance
	r.roundsBox.Hide()
	r.ExtendBaseWidget(r)

	header := container.NewStack(
		r.background,
		container.NewHBox(
			r.expandBtn,
			r.label,
			layout.NewSpacer(),
			r.editBtn,
			r.delBtn,
		),
	)
	r.content = container.NewVBox(header, r.roundsBox)
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

// HistoryTab manages the game history view with CRUD operations
type HistoryTab struct {
	db             *sql.DB
	window         fyne.Window
	listBox        *fyne.Container // plain VBox of selectableRows; allows variable heights
	rows           []*selectableRow
	games          []database.GameStats
	selected       map[int]bool
	expanded       map[int]bool
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
		expanded:       make(map[int]bool),
		lastClickedIdx: -1,
	}
	h.refresh()
	return h
}

// Container returns the tab content
func (h *HistoryTab) Container() fyne.CanvasObject {
	h.listBox = container.NewVBox()
	h.refreshRows()

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

	scroll := container.NewVScroll(h.listBox)
	return container.NewBorder(toolbar, nil, nil, nil, scroll)
}

// refreshRows rebuilds the list of selectableRow widgets to match h.games and
// the current expanded/selected state. Used in place of widget.List so that
// rows may have variable heights without recycling glitches.
func (h *HistoryTab) refreshRows() {
	if h.listBox == nil {
		return
	}
	// Grow/shrink the row slice to match game count.
	for len(h.rows) < len(h.games) {
		h.rows = append(h.rows, newSelectableRow(h))
	}
	if len(h.rows) > len(h.games) {
		h.rows = h.rows[:len(h.games)]
	}

	h.listBox.Objects = h.listBox.Objects[:0]
	for i := range h.games {
		game := h.games[i]
		gameID := game.ID
		row := h.rows[i]
		row.rowIdx = i
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
		row.label.SetText(fmt.Sprintf("%s | CT:%d T:%d (max:%d) %s [%s]",
			dateStr, game.CTScore, game.TScore, game.GameScore, winner, teamStr))

		row.SetSelected(h.selected[gameID])

		if len(h.selected) > 1 {
			row.editBtn.Disable()
		} else {
			row.editBtn.Enable()
		}

		gameCopy := game
		row.editBtn.OnTapped = func() {
			if len(h.selected) <= 1 {
				h.showEditDialog(&gameCopy)
			}
		}
		row.delBtn.OnTapped = func() { h.confirmDelete(&gameCopy) }
		row.expandBtn.OnTapped = func() { h.toggleExpanded(gameID) }

		if h.expanded[gameID] {
			row.expandBtn.SetText("▾")
			h.populateRounds(row, game)
			row.roundsBox.Show()
		} else {
			row.expandBtn.SetText("▸")
			row.roundsBox.Hide()
		}

		h.listBox.Add(row)
	}
	h.listBox.Refresh()
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
	h.refreshRows()
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
	h.refreshRows()
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

// toggleExpanded flips the expansion state for a game row and re-renders.
func (h *HistoryTab) toggleExpanded(gameID int) {
	if h.expanded[gameID] {
		delete(h.expanded, gameID)
	} else {
		h.expanded[gameID] = true
	}
	h.refreshRows()
}

// populateRounds fills a row's rounds panel with one canvas.Text per round.
// Using canvas.Text directly (rather than widget.Label) avoids widget padding
// and gives a denser list.
func (h *HistoryTab) populateRounds(row *selectableRow, game database.GameStats) {
	rounds, err := database.GetRoundsForGame(context.Background(), h.db, game.ID)
	row.roundsBox.Objects = row.roundsBox.Objects[:0]
	if err != nil {
		row.roundsBox.Add(widget.NewLabel(fmt.Sprintf("Error loading rounds: %v", err)))
		row.roundsBox.Refresh()
		return
	}
	if len(rounds) == 0 {
		row.roundsBox.Add(widget.NewLabel("No round timestamps recorded for this game."))
		row.roundsBox.Refresh()
		return
	}
	textColor := theme.Color(theme.ColorNameForeground)
	for i, r := range rounds {
		outcome := "draw"
		switch game.Team {
		case database.TeamCT:
			if r.Winner == database.TeamCT {
				outcome = "won"
			} else {
				outcome = "lost"
			}
		case database.TeamT:
			if r.Winner == database.TeamT {
				outcome = "won"
			} else {
				outcome = "lost"
			}
		}
		t := canvas.NewText(fmt.Sprintf(
			"  #%d  %s  %s won  (%s)",
			i+1,
			r.CreatedAt.Format("15:04:05"),
			r.Winner,
			outcome,
		), textColor)
		t.TextSize = theme.TextSize()
		row.roundsBox.Add(t)
	}
	row.roundsBox.Refresh()
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
	h.refreshRows()
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
		if _, err := database.SaveGame(ctx, h.db, ct, t, max, team); err != nil {
			dialog.ShowError(err, h.window)
			return
		}
		h.refresh()
		if h.onUpdate != nil {
			h.onUpdate()
		}
	}, h.window)
}

// roundEdit tracks a single round inside the edit dialog. Changes are staged
// and applied as a batch when the user hits Save.
type roundEdit struct {
	id        int            // 0 for newly added rounds
	winner    database.Team
	createdAt time.Time
	deleted   bool
	dirty     bool // existing round whose winner was changed
}

func (h *HistoryTab) showEditDialog(game *database.GameStats) {
	maxEntry := widget.NewEntry()
	maxEntry.SetText(strconv.Itoa(game.GameScore))
	teamSelect := widget.NewSelect([]string{"None", "CT", "T"}, nil)
	if game.Team == "" {
		teamSelect.SetSelected("None")
	} else {
		teamSelect.SetSelected(string(game.Team))
	}

	form := widget.NewForm(
		widget.NewFormItem("Game Score", maxEntry),
		widget.NewFormItem("Player's Team", teamSelect),
	)

	// Load existing rounds into a working copy.
	ctx := context.Background()
	existing, err := database.GetRoundsForGame(ctx, h.db, game.ID)
	if err != nil {
		dialog.ShowError(err, h.window)
		return
	}
	edits := make([]*roundEdit, 0, len(existing))
	for _, r := range existing {
		edits = append(edits, &roundEdit{
			id:        r.ID,
			winner:    r.Winner,
			createdAt: r.CreatedAt,
		})
	}

	roundsList := container.NewVBox()
	var rebuildRounds func()
	rebuildRounds = func() {
		roundsList.Objects = roundsList.Objects[:0]
		visibleIdx := 0
		for _, e := range edits {
			if e.deleted {
				continue
			}
			visibleIdx++
			edit := e
			num := widget.NewLabel(fmt.Sprintf("#%d", visibleIdx))
			ts := widget.NewLabel(edit.createdAt.Format("15:04:05"))
			sel := widget.NewSelect([]string{"CT", "T"}, func(v string) {
				if database.Team(v) != edit.winner {
					edit.winner = database.Team(v)
					edit.dirty = true
				}
			})
			sel.SetSelected(string(edit.winner))
			del := widget.NewButton("×", func() {
				edit.deleted = true
				rebuildRounds()
			})
			del.Importance = widget.DangerImportance
			roundsList.Add(container.NewBorder(nil, nil,
				container.NewHBox(num, ts),
				del,
				sel,
			))
		}
		if visibleIdx == 0 {
			hint := widget.NewLabel("No rounds recorded.")
			hint.Alignment = fyne.TextAlignCenter
			roundsList.Add(hint)
		}
		roundsList.Refresh()
	}
	rebuildRounds()

	addCTBtn := widget.NewButton("+ CT Round", func() {
		edits = append(edits, &roundEdit{winner: database.TeamCT, createdAt: time.Now()})
		rebuildRounds()
	})
	addTBtn := widget.NewButton("+ T Round", func() {
		edits = append(edits, &roundEdit{winner: database.TeamT, createdAt: time.Now()})
		rebuildRounds()
	})

	addButtons := container.NewHBox(layout.NewSpacer(), addCTBtn, addTBtn, layout.NewSpacer())

	// Keep a baseline visible area for the round list so the dialog feels
	// consistent regardless of round count; scrolling kicks in past this.
	roundsScroll := container.NewVScroll(roundsList)
	roundsScroll.SetMinSize(fyne.NewSize(360, 180))

	content := container.NewVBox(
		form,
		widget.NewSeparator(),
		widget.NewLabel("Rounds:"),
		roundsScroll,
		addButtons,
	)

	d := dialog.NewCustomConfirm("Edit Game", "Save", "Cancel", content, func(save bool) {
		if !save {
			return
		}
		max, _ := strconv.Atoi(maxEntry.Text)
		if max <= 0 {
			max = 8
		}
		team := database.TeamNone
		if teamSelect.Selected != "None" {
			team = database.Team(teamSelect.Selected)
		}

		// CT/T scores are derived from the (possibly edited) round list so
		// game_stats stays consistent with rounds.
		ct, t := 0, 0
		for _, e := range edits {
			if e.deleted {
				continue
			}
			switch e.winner {
			case database.TeamCT:
				ct++
			case database.TeamT:
				t++
			}
		}

		if err := database.UpdateGame(ctx, h.db, game.ID, ct, t, max, team); err != nil {
			dialog.ShowError(err, h.window)
			return
		}
		for _, e := range edits {
			switch {
			case e.id == 0 && !e.deleted:
				if err := database.InsertRoundForGame(ctx, h.db, game.ID, e.winner, team); err != nil {
					dialog.ShowError(err, h.window)
					return
				}
			case e.id != 0 && e.deleted:
				if err := database.DeleteRound(ctx, h.db, e.id); err != nil {
					dialog.ShowError(err, h.window)
					return
				}
			case e.id != 0 && e.dirty:
				if err := database.UpdateRoundWinner(ctx, h.db, e.id, e.winner); err != nil {
					dialog.ShowError(err, h.window)
					return
				}
			}
		}
		h.refresh()
		if h.onUpdate != nil {
			h.onUpdate()
		}
	}, h.window)
	d.Resize(fyne.NewSize(460, 0))
	d.Show()
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
