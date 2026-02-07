package tracker

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/dialog"

	"csstatstracker/internal/config"
	"csstatstracker/internal/database"
	"csstatstracker/internal/hotkey"
	"csstatstracker/internal/sound"
	"csstatstracker/internal/ui"
)

// Tracker manages the game state and UI updates
type Tracker struct {
	ctWins       int
	tWins        int
	team         database.Team
	ctLabel      *canvas.Text
	tLabel       *canvas.Text
	maxEntry     *ui.AutoSizeEntry
	db           *sql.DB
	window       fyne.Window
	Config       *config.Config
	hotkey       *hotkey.Handler
	sound        *sound.Player
	onTeamChange func(database.Team)
}

// New creates a new Tracker instance
func New(db *sql.DB, w fyne.Window, cfg *config.Config, ctLabel, tLabel *canvas.Text, soundFS embed.FS) *Tracker {
	t := &Tracker{
		ctWins:  0,
		tWins:   0,
		ctLabel: ctLabel,
		tLabel:  tLabel,
		db:      db,
		window:  w,
		Config:  cfg,
		sound:   sound.New(soundFS, cfg.SoundEnabled, cfg.SoundVolume),
	}

	// Create entry with auto-save on change
	maxEntry := ui.NewAutoSizeEntry()
	maxEntry.SetPlaceHolder("8")
	maxEntry.Text = strconv.Itoa(cfg.GameScore)
	maxEntry.OnChanged = func(value string) {
		t.saveGameScoreConfig(value)
		maxEntry.Refresh()
	}
	t.maxEntry = maxEntry

	// Setup hotkeys
	bindings := &hotkey.Bindings{
		IncrementCT: cfg.Hotkeys.IncrementCT,
		DecrementCT: cfg.Hotkeys.DecrementCT,
		IncrementT:  cfg.Hotkeys.IncrementT,
		DecrementT:  cfg.Hotkeys.DecrementT,
		Reset:       cfg.Hotkeys.Reset,
		SelectCT:    cfg.Hotkeys.SelectCT,
		SelectT:     cfg.Hotkeys.SelectT,
		SwapTeams:   cfg.Hotkeys.SwapTeams,
	}
	t.hotkey = hotkey.NewHandler(bindings)

	return t
}

// MaxEntry returns the game score entry widget
func (t *Tracker) MaxEntry() *ui.AutoSizeEntry {
	return t.maxEntry
}

// StartHotkeys begins listening for global hotkey events
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
			case hotkey.ActionReset:
				t.Reset()
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

// Sound returns the sound player
func (t *Tracker) Sound() *sound.Player {
	return t.sound
}

// SetTeam sets the player's team
func (t *Tracker) SetTeam(team database.Team) {
	t.team = team
}

// Team returns the current team
func (t *Tracker) Team() database.Team {
	return t.team
}

// UpdateHotkeys updates the hotkey bindings
func (t *Tracker) UpdateHotkeys() {
	bindings := &hotkey.Bindings{
		IncrementCT: t.Config.Hotkeys.IncrementCT,
		DecrementCT: t.Config.Hotkeys.DecrementCT,
		IncrementT:  t.Config.Hotkeys.IncrementT,
		DecrementT:  t.Config.Hotkeys.DecrementT,
		Reset:       t.Config.Hotkeys.Reset,
		SelectCT:    t.Config.Hotkeys.SelectCT,
		SelectT:     t.Config.Hotkeys.SelectT,
		SwapTeams:   t.Config.Hotkeys.SwapTeams,
	}
	t.hotkey.UpdateBindings(bindings)
}

// SetOnTeamChange sets the callback for team changes
func (t *Tracker) SetOnTeamChange(callback func(database.Team)) {
	t.onTeamChange = callback
}

// SelectCT selects the CT team
func (t *Tracker) SelectCT() {
	t.team = database.TeamCT
	t.sound.PlayCTSelect()
	if t.onTeamChange != nil {
		fyne.Do(func() {
			t.onTeamChange(database.TeamCT)
		})
	}
}

// SelectT selects the T team
func (t *Tracker) SelectT() {
	t.team = database.TeamT
	t.sound.PlayTSelect()
	if t.onTeamChange != nil {
		fyne.Do(func() {
			t.onTeamChange(database.TeamT)
		})
	}
}

// SwapTeams swaps the player's team and counter values
func (t *Tracker) SwapTeams() {
	// Swap counter values
	t.ctWins, t.tWins = t.tWins, t.ctWins
	t.updateLabels()

	// Swap team
	switch t.team {
	case database.TeamCT:
		t.team = database.TeamT
		t.sound.PlayTSelect()
		if t.onTeamChange != nil {
			fyne.Do(func() {
				t.onTeamChange(database.TeamT)
			})
		}
	case database.TeamT:
		t.team = database.TeamCT
		t.sound.PlayCTSelect()
		if t.onTeamChange != nil {
			fyne.Do(func() {
				t.onTeamChange(database.TeamCT)
			})
		}
	default:
		// No team selected, just swap counters without sound
	}
}

func (t *Tracker) saveGameScoreConfig(value string) {
	newScore, err := strconv.Atoi(value)
	if err != nil || newScore <= 0 {
		return
	}

	if newScore != t.Config.GameScore {
		t.Config.GameScore = newScore
		_ = config.Save(t.Config, config.DefaultConfigFile)
	}
}

// IncrementCT increases the CT score
func (t *Tracker) IncrementCT() {
	gameScore, err := strconv.Atoi(t.maxEntry.Text)
	if err != nil || gameScore <= 0 {
		return
	}
	if t.ctWins >= gameScore {
		return
	}
	t.ctWins++
	t.updateLabels()
	t.sound.PlayCTIncrement()
	t.checkAutoSave()
}

// DecrementCT decreases the CT score
func (t *Tracker) DecrementCT() {
	if t.ctWins > 0 {
		t.ctWins--
		t.updateLabels()
		t.sound.PlayCTDecrement()
	}
}

// IncrementT increases the T score
func (t *Tracker) IncrementT() {
	gameScore, err := strconv.Atoi(t.maxEntry.Text)
	if err != nil || gameScore <= 0 {
		return
	}
	if t.tWins >= gameScore {
		return
	}
	t.tWins++
	t.updateLabels()
	t.sound.PlayTIncrement()
	t.checkAutoSave()
}

// DecrementT decreases the T score
func (t *Tracker) DecrementT() {
	if t.tWins > 0 {
		t.tWins--
		t.updateLabels()
		t.sound.PlayTDecrement()
	}
}

// checkAutoSave saves the game automatically if a team reaches the max score
func (t *Tracker) checkAutoSave() {
	gameScore, err := strconv.Atoi(t.maxEntry.Text)
	if err != nil || gameScore <= 0 {
		return
	}

	if t.ctWins >= gameScore || t.tWins >= gameScore {
		t.HandleDone()
	}
}

// Reset clears both scores
func (t *Tracker) Reset() {
	t.ctWins = 0
	t.tWins = 0
	t.updateLabels()
	t.sound.PlayReset()
}

func (t *Tracker) updateLabels() {
	fyne.Do(func() {
		t.ctLabel.Text = fmt.Sprintf("%d", t.ctWins)
		t.tLabel.Text = fmt.Sprintf("%d", t.tWins)
		t.ctLabel.Refresh()
		t.tLabel.Refresh()
	})
}

// HandleDone saves the game and resets
func (t *Tracker) HandleDone() {
	ctx := context.Background()

	gameScore, err := strconv.Atoi(t.maxEntry.Text)
	if err != nil || gameScore <= 0 {
		dialog.ShowError(fmt.Errorf("invalid game score: must be a positive number"), t.window)
		return
	}

	err = database.SaveGame(ctx, t.db, t.ctWins, t.tWins, gameScore, t.team)
	if err != nil {
		dialog.ShowError(err, t.window)
		return
	}

	t.playMatchEndSound()
	t.resetScores()
}

// playMatchEndSound plays win, lose, or generic match end sound based on team
func (t *Tracker) playMatchEndSound() {
	switch t.team {
	case database.TeamCT:
		if t.ctWins > t.tWins {
			t.sound.PlayWin()
		} else {
			t.sound.PlayLose()
		}
	case database.TeamT:
		if t.tWins > t.ctWins {
			t.sound.PlayWin()
		} else {
			t.sound.PlayLose()
		}
	default:
		t.sound.PlayMatchEnd()
	}
}

// resetScores clears scores without playing sound (used by HandleDone)
func (t *Tracker) resetScores() {
	t.ctWins = 0
	t.tWins = 0
	t.updateLabels()
}
