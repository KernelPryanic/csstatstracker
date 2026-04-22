//go:build linux || windows

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	csstatstracker "csstatstracker"
	"csstatstracker/internal/config"
	"csstatstracker/internal/database"
	"csstatstracker/internal/singleinstance"
	"csstatstracker/internal/tracker"
	"csstatstracker/internal/ui"
)

// singleInstancePort is a fixed loopback port used as a cross-platform mutex.
const singleInstancePort = 53017

// logFilter wraps an io.Writer and drops lines matching the systray "not ready"
// warning that Fyne prints during startup — the icon gets set correctly once
// the backend is ready, so the warning is pure noise.
type logFilter struct {
	inner interface{ Write([]byte) (int, error) }
}

func (f *logFilter) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("systray error: unable to set icon: tray not ready yet")) {
		return len(p), nil
	}
	return f.inner.Write(p)
}

func init() {
	log.SetOutput(&logFilter{inner: os.Stderr})
}

func main() {
	lock, err := singleinstance.Acquire(singleInstancePort)
	if err != nil {
		if errors.Is(err, singleinstance.ErrAlreadyRunning) {
			// Show a GUI dialog because the binary uses -H=windowsgui and has
			// no console — a silent exit would leave the user wondering why.
			notifyAlreadyRunning()
			os.Exit(1)
		}
		panic(fmt.Errorf("failed to acquire single-instance lock: %w", err))
	}
	defer lock.Release()

	ctx := context.Background()

	// Load configuration
	cfg, err := config.Load(config.DefaultConfigFile)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}

	db, err := database.Init(ctx, database.DefaultDBFile, csstatstracker.MigrationsFS)
	if err != nil {
		panic(fmt.Errorf("failed to initialize database: %w", err))
	}
	defer func() { _ = db.Close() }()

	a := app.New()
	w := a.NewWindow("CS Stats Tracker")

	// Create counter labels
	ctLabel := canvas.NewText("0", color.RGBA{R: 100, G: 149, B: 237, A: 255})
	ctLabel.TextSize = 72
	ctLabel.Alignment = fyne.TextAlignCenter

	tLabel := canvas.NewText("0", color.RGBA{R: 255, G: 140, B: 0, A: 255})
	tLabel.TextSize = 72
	tLabel.Alignment = fyne.TextAlignCenter

	t := tracker.New(db, w, cfg, ctLabel, tLabel, csstatstracker.SoundFS)

	// Create CT side (left)
	ctTitle := canvas.NewText("CT", color.RGBA{R: 100, G: 149, B: 237, A: 255})
	ctTitle.TextSize = 32
	ctTitle.Alignment = fyne.TextAlignCenter

	ctPlusButton := widget.NewButton("+", func() {
		t.IncrementCT()
	})
	ctPlusButton.Importance = widget.HighImportance

	ctMinusButton := widget.NewButton("-", func() {
		t.DecrementCT()
	})
	ctMinusButton.Importance = widget.WarningImportance

	ctButtonsContainer := container.NewGridWithColumns(2,
		ctPlusButton,
		ctMinusButton,
	)

	ctContainer := container.NewBorder(
		ctTitle,
		ctButtonsContainer,
		nil,
		nil,
		container.NewCenter(ctLabel),
	)

	// Create T side (right)
	tTitle := canvas.NewText("T", color.RGBA{R: 255, G: 140, B: 0, A: 255})
	tTitle.TextSize = 32
	tTitle.Alignment = fyne.TextAlignCenter

	tPlusButton := widget.NewButton("+", func() {
		t.IncrementT()
	})
	tPlusButton.Importance = widget.HighImportance

	tMinusButton := widget.NewButton("-", func() {
		t.DecrementT()
	})
	tMinusButton.Importance = widget.WarningImportance

	tButtonsContainer := container.NewGridWithColumns(2,
		tPlusButton,
		tMinusButton,
	)

	tContainer := container.NewBorder(
		tTitle,
		tButtonsContainer,
		nil,
		nil,
		container.NewCenter(tLabel),
	)

	// Create side-by-side layout
	countersContainer := container.NewGridWithColumns(2,
		ctContainer,
		tContainer,
	)

	// Create team selection
	teamSelect := widget.NewSelect([]string{"None", "CT", "T"}, func(selected string) {
		if selected == "None" {
			t.SetTeam(database.TeamNone)
		} else {
			t.SetTeam(database.Team(selected))
		}
	})
	teamSelect.SetSelected("None")

	// Wire up hotkey team selection to update UI
	t.SetOnTeamChange(func(team database.Team) {
		switch team {
		case database.TeamCT:
			teamSelect.SetSelected("CT")
		case database.TeamT:
			teamSelect.SetSelected("T")
		default:
			teamSelect.SetSelected("None")
		}
	})

	// Team selector row.
	teamRow := container.NewHBox(
		layout.NewSpacer(),
		widget.NewLabel("Team:"),
		teamSelect,
		layout.NewSpacer(),
	)

	// Action buttons row.
	swapButton := widget.NewButton("Swap Teams", func() {
		t.SwapTeams()
	})
	actionButtonsContainer := container.NewHBox(
		layout.NewSpacer(),
		swapButton,
		layout.NewSpacer(),
	)

	// Tracker tab content
	trackerContent := container.NewBorder(
		nil,
		container.NewVBox(
			teamRow,
			actionButtonsContainer,
		),
		nil,
		nil,
		countersContainer,
	)

	// Create history tab
	statsTab := ui.NewStatsTab(db, w, cfg, func() {
		if err := config.Save(cfg, config.DefaultConfigFile); err != nil {
			fyne.LogError("Failed to save config", err)
		}
	})
	historyTab := ui.NewHistoryTab(db, w, func() {
		statsTab.Refresh()
	})

	// Create settings tab
	settingsTab := ui.NewSettingsTab(t.Config, w, func(cfg *config.Config) {
		if err := config.Save(cfg, config.DefaultConfigFile); err != nil {
			fyne.LogError("Failed to save config", err)
		}
		t.UpdateHotkeys()
		t.Sound().SetEnabled(cfg.SoundEnabled)
		t.Sound().SetVolume(cfg.SoundVolume)
	})

	// Create tabs
	historyTabItem := container.NewTabItem("History", historyTab.Container())
	statsTabItem := container.NewTabItem("Stats", statsTab.Container())
	tabs := container.NewAppTabs(
		container.NewTabItem("Tracker", trackerContent),
		historyTabItem,
		statsTabItem,
		container.NewTabItem("Settings", settingsTab.Container()),
	)

	// Auto-refresh tabs when switching to them
	tabs.OnSelected = func(tab *container.TabItem) {
		switch tab {
		case historyTabItem:
			historyTab.Refresh()
		case statsTabItem:
			statsTab.Refresh()
		}
	}

	w.SetContent(tabs)
	w.Resize(fyne.Size{Width: 600, Height: 450})

	// Setup system tray. Also set the icon as the app's main icon so the
	// systray has a fallback if the first SetSystemTrayIcon call races with
	// the systray backend starting up.
	trayIcon := fyne.NewStaticResource("icon.png", csstatstracker.IconData)
	a.SetIcon(trayIcon)
	if desk, ok := a.(desktop.App); ok {
		desk.SetSystemTrayIcon(trayIcon)

		trayMenu := fyne.NewMenu("CS Stats Tracker",
			fyne.NewMenuItem("Show", func() { w.Show() }),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", func() { a.Quit() }),
		)
		desk.SetSystemTrayMenu(trayMenu)
	}

	// Intercept window close to minimize to tray if enabled
	w.SetCloseIntercept(func() {
		if cfg.MinimizeToTray {
			w.Hide()
		} else {
			a.Quit()
		}
	})

	// Start hotkey handling
	t.StartHotkeys()

	w.ShowAndRun()
}
