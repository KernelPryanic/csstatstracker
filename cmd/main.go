//go:build linux || windows

package main

import (
	"context"
	"fmt"
	"image/color"

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
	"csstatstracker/internal/tracker"
	"csstatstracker/internal/ui"
)

func main() {
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

	// Create game score input
	gameScoreLabel := widget.NewLabel("Game:")
	gameScoreContainer := container.NewHBox(
		layout.NewSpacer(),
		gameScoreLabel,
		t.MaxEntry(),
		widget.NewLabel("Team:"),
		teamSelect,
		layout.NewSpacer(),
	)

	// Create action buttons
	resetButton := widget.NewButton("Reset", func() {
		t.Reset()
	})
	resetButton.Importance = widget.DangerImportance

	actionButtonsContainer := container.NewHBox(
		layout.NewSpacer(),
		resetButton,
		layout.NewSpacer(),
	)

	// Tracker tab content
	trackerContent := container.NewBorder(
		nil,
		container.NewVBox(
			gameScoreContainer,
			actionButtonsContainer,
		),
		nil,
		nil,
		countersContainer,
	)

	// Create history tab
	statsTab := ui.NewStatsTab(db, w)
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

	// Setup system tray
	if desk, ok := a.(desktop.App); ok {
		// Create tray icon from embedded resource
		trayIcon := fyne.NewStaticResource("icon.png", csstatstracker.IconData)
		desk.SetSystemTrayIcon(trayIcon)

		// Create tray menu
		trayMenu := fyne.NewMenu("CS Stats Tracker",
			fyne.NewMenuItem("Show", func() {
				w.Show()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("Quit", func() {
				a.Quit()
			}),
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
