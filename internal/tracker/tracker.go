package tracker

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"csstatstracker/internal/config"
	"csstatstracker/internal/database"
	"csstatstracker/internal/hotkey"
	"csstatstracker/internal/sound"
)

// Tracker owns the on-screen counters and records each increment as a round
// in the database. There is no concept of a "game" — counters are purely a
// visual running total since app start.
type Tracker struct {
	ctWins       int
	tWins        int
	team         database.Team
	ctLabel      *canvas.Text
	tLabel       *canvas.Text
	db           *sql.DB
	window       fyne.Window
	Config       *config.Config
	hotkey       *hotkey.Handler
	sound        *sound.Player
	onTeamChange func(database.Team)
}

// New creates a new Tracker instance.
func New(db *sql.DB, w fyne.Window, cfg *config.Config, ctLabel, tLabel *canvas.Text, soundFS embed.FS) *Tracker {
	t := &Tracker{
		ctLabel: ctLabel,
		tLabel:  tLabel,
		db:      db,
		window:  w,
		Config:  cfg,
		sound:   sound.New(soundFS, cfg.SoundEnabled, cfg.SoundVolume),
	}

	bindings := &hotkey.Bindings{
		IncrementCT: cfg.Hotkeys.IncrementCT,
		DecrementCT: cfg.Hotkeys.DecrementCT,
		IncrementT:  cfg.Hotkeys.IncrementT,
		DecrementT:  cfg.Hotkeys.DecrementT,
		SelectCT:    cfg.Hotkeys.SelectCT,
		SelectT:     cfg.Hotkeys.SelectT,
		SwapTeams:   cfg.Hotkeys.SwapTeams,
	}
	t.hotkey = hotkey.NewHandler(bindings)

	return t
}

// StartHotkeys begins listening for global hotkey events.
func (t *Tracker) StartHotkeys() {
	t.hotkey.Start()

	go func() {
		for action := range t.hotkey.Actions() {
			switch action {
			case hotkey.ActionIncrementCT:
				t.IncrementCT()
			case hotkey.ActionDecrementCT:
				t.DecrementCT()
			case hotkey.ActionIncrementT:
				t.IncrementT()
			case hotkey.ActionDecrementT:
				t.DecrementT()
			case hotkey.ActionSelectCT:
				t.SelectCT()
			case hotkey.ActionSelectT:
				t.SelectT()
			case hotkey.ActionSwapTeams:
				t.SwapTeams()
			}
		}
	}()
}

// Sound returns the sound player.
func (t *Tracker) Sound() *sound.Player { return t.sound }

// SetTeam sets the player's team.
func (t *Tracker) SetTeam(team database.Team) { t.team = team }

// Team returns the current team.
func (t *Tracker) Team() database.Team { return t.team }

// UpdateHotkeys updates the hotkey bindings.
func (t *Tracker) UpdateHotkeys() {
	bindings := &hotkey.Bindings{
		IncrementCT: t.Config.Hotkeys.IncrementCT,
		DecrementCT: t.Config.Hotkeys.DecrementCT,
		IncrementT:  t.Config.Hotkeys.IncrementT,
		DecrementT:  t.Config.Hotkeys.DecrementT,
		SelectCT:    t.Config.Hotkeys.SelectCT,
		SelectT:     t.Config.Hotkeys.SelectT,
		SwapTeams:   t.Config.Hotkeys.SwapTeams,
	}
	t.hotkey.UpdateBindings(bindings)
}

// SetOnTeamChange sets the callback for team changes.
func (t *Tracker) SetOnTeamChange(callback func(database.Team)) {
	t.onTeamChange = callback
}

// SelectCT selects CT as the player's team.
func (t *Tracker) SelectCT() {
	t.team = database.TeamCT
	t.sound.PlayCTSelect()
	if t.onTeamChange != nil {
		fyne.Do(func() { t.onTeamChange(database.TeamCT) })
	}
}

// SelectT selects T as the player's team.
func (t *Tracker) SelectT() {
	t.team = database.TeamT
	t.sound.PlayTSelect()
	if t.onTeamChange != nil {
		fyne.Do(func() { t.onTeamChange(database.TeamT) })
	}
}

// SwapTeams flips the player's team. Counters stay as-is — they just reflect
// rounds recorded so far, unrelated to which side the player is on now.
func (t *Tracker) SwapTeams() {
	switch t.team {
	case database.TeamCT:
		t.team = database.TeamT
		t.sound.PlayTSelect()
		if t.onTeamChange != nil {
			fyne.Do(func() { t.onTeamChange(database.TeamT) })
		}
	case database.TeamT:
		t.team = database.TeamCT
		t.sound.PlayCTSelect()
		if t.onTeamChange != nil {
			fyne.Do(func() { t.onTeamChange(database.TeamCT) })
		}
	}
}

// IncrementCT records a CT round.
func (t *Tracker) IncrementCT() {
	t.ctWins++
	t.recordRound(database.TeamCT)
	t.updateLabels()
	t.sound.PlayCTIncrement()
}

// DecrementCT deletes the most recent CT round.
func (t *Tracker) DecrementCT() {
	if t.ctWins > 0 {
		t.ctWins--
		t.undoLastRound(database.TeamCT)
		t.updateLabels()
		t.sound.PlayCTDecrement()
	}
}

// IncrementT records a T round.
func (t *Tracker) IncrementT() {
	t.tWins++
	t.recordRound(database.TeamT)
	t.updateLabels()
	t.sound.PlayTIncrement()
}

// DecrementT deletes the most recent T round.
func (t *Tracker) DecrementT() {
	if t.tWins > 0 {
		t.tWins--
		t.undoLastRound(database.TeamT)
		t.updateLabels()
		t.sound.PlayTDecrement()
	}
}

func (t *Tracker) recordRound(winner database.Team) {
	if _, err := database.InsertRound(context.Background(), t.db, winner, t.team); err != nil {
		fyne.LogError("failed to record round", err)
	}
}

func (t *Tracker) undoLastRound(winner database.Team) {
	if _, err := database.DeleteLastRoundForWinner(context.Background(), t.db, winner); err != nil {
		fyne.LogError("failed to undo round", err)
	}
}

func (t *Tracker) updateLabels() {
	fyne.Do(func() {
		t.ctLabel.Text = fmt.Sprintf("%d", t.ctWins)
		t.tLabel.Text = fmt.Sprintf("%d", t.tWins)
		t.ctLabel.Refresh()
		t.tLabel.Refresh()
	})
}
