package ui

import (
	"context"
	"database/sql"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"csstatstracker/internal/config"
	"csstatstracker/internal/database"
)

// AggregationInterval defines how to group stats in the chart
type AggregationInterval int

const (
	AggregateByDay AggregationInterval = iota
	AggregateByWeek
	AggregateByMonth
	AggregateByYear
)

// StatsScope selects whether the Win Rate sub-tab counts games or rounds.
type StatsScope int

const (
	ScopeGames StatsScope = iota
	ScopeRounds
)

// StatsTab manages the statistics view
type StatsTab struct {
	db            *sql.DB
	window        fyne.Window
	cfg           *config.Config
	onSave        func()
	currentWindow database.TimeWindow
	aggregation   AggregationInterval
	scope         StatsScope
	container     *fyne.Container

	// Sub-tabs
	subTabs *container.AppTabs

	// Win Rate sub-tab
	winRateLabel   *widget.Label
	ctWinRateLabel *widget.Label
	tWinRateLabel  *widget.Label
	gamesLabel     *widget.Label
	chartLabel     *widget.Label
	chartContainer *fyne.Container

	// Play Time sub-tab
	totalTimeLabel     *widget.Label
	ctTimeLabel        *widget.Label
	tTimeLabel         *widget.Label
	timeChartLabel     *widget.Label
	timeChartContainer *fyne.Container
}

// NewStatsTab creates a new statistics tab
func NewStatsTab(db *sql.DB, window fyne.Window, cfg *config.Config, onSave func()) *StatsTab {
	s := &StatsTab{
		db:     db,
		window: window,
		cfg:    cfg,
		onSave: onSave,
	}

	// Initialize from config
	s.currentWindow = s.periodToWindow(cfg.StatsPeriod)
	s.aggregation = s.groupToAggregation(cfg.StatsGroup)
	s.scope = s.scopeFromString(cfg.StatsScope)

	return s
}

// scopeFromString converts a config string to StatsScope.
func (s *StatsTab) scopeFromString(v string) StatsScope {
	if v == "Rounds" {
		return ScopeRounds
	}
	return ScopeGames
}

// periodToWindow converts a period string to TimeWindow
func (s *StatsTab) periodToWindow(period string) database.TimeWindow {
	switch period {
	case "Day":
		return database.WindowDay
	case "Week":
		return database.WindowWeek
	case "Month":
		return database.WindowMonth
	case "Year":
		return database.WindowYear
	default:
		return database.WindowAll
	}
}

// groupToAggregation converts a group string to AggregationInterval
func (s *StatsTab) groupToAggregation(group string) AggregationInterval {
	switch group {
	case "By Week":
		return AggregateByWeek
	case "By Month":
		return AggregateByMonth
	case "By Year":
		return AggregateByYear
	default:
		return AggregateByDay
	}
}

// Container returns the tab content
func (s *StatsTab) Container() fyne.CanvasObject {
	// Initialize labels for Win Rate sub-tab
	s.winRateLabel = widget.NewLabel("Win Rate: --")
	s.ctWinRateLabel = widget.NewLabel("CT Win Rate: --")
	s.tWinRateLabel = widget.NewLabel("T Win Rate: --")
	s.gamesLabel = widget.NewLabel("Games: 0")
	s.chartLabel = widget.NewLabel("Net Wins/Losses by Day:")
	s.chartContainer = container.NewStack()

	// Initialize labels for Play Time sub-tab
	s.totalTimeLabel = widget.NewLabel("Total Play Time: --")
	s.ctTimeLabel = widget.NewLabel("CT Play Time: --")
	s.tTimeLabel = widget.NewLabel("T Play Time: --")
	s.timeChartLabel = widget.NewLabel("Play Time by Day:")
	s.timeChartContainer = container.NewStack()

	// Time window selector
	windowSelect := widget.NewSelect(
		[]string{"Day", "Week", "Month", "Year", "All Time"},
		func(selected string) {
			s.currentWindow = s.periodToWindow(selected)
			s.cfg.StatsPeriod = selected
			if s.onSave != nil {
				s.onSave()
			}
			s.refresh()
		},
	)
	windowSelect.SetSelected(s.cfg.StatsPeriod)

	// Aggregation selector
	aggregationSelect := widget.NewSelect(
		[]string{"By Day", "By Week", "By Month", "By Year"},
		func(selected string) {
			s.aggregation = s.groupToAggregation(selected)
			s.cfg.StatsGroup = selected
			if s.onSave != nil {
				s.onSave()
			}
			s.updateChartLabels()
			s.refresh()
		},
	)
	aggregationSelect.SetSelected(s.cfg.StatsGroup)

	// Shared controls (Period and Group)
	controlsPanel := container.NewHBox(
		widget.NewLabel("Period:"),
		windowSelect,
		widget.NewLabel("Group:"),
		aggregationSelect,
	)

	// Scope selector lives on the Win Rate sub-tab only (Play Time is always
	// game-based).
	scopeSelect := widget.NewSelect([]string{"Games", "Rounds"}, func(selected string) {
		s.scope = s.scopeFromString(selected)
		s.cfg.StatsScope = selected
		if s.onSave != nil {
			s.onSave()
		}
		s.refresh()
	})
	scopeSelect.SetSelected(s.cfg.StatsScope)

	scopePanel := container.NewHBox(
		widget.NewLabel("Scope:"),
		scopeSelect,
	)

	// Win Rate sub-tab content
	winRateContent := container.NewBorder(
		container.NewVBox(
			scopePanel,
			widget.NewSeparator(),
			s.gamesLabel,
			s.winRateLabel,
			widget.NewSeparator(),
			widget.NewLabel("Win Rate by Team:"),
			s.ctWinRateLabel,
			s.tWinRateLabel,
			widget.NewSeparator(),
			s.chartLabel,
		),
		nil, nil, nil,
		s.chartContainer,
	)

	// Play Time sub-tab content
	playTimeContent := container.NewBorder(
		container.NewVBox(
			widget.NewSeparator(),
			s.totalTimeLabel,
			widget.NewSeparator(),
			widget.NewLabel("Play Time by Team:"),
			s.ctTimeLabel,
			s.tTimeLabel,
			widget.NewSeparator(),
			s.timeChartLabel,
		),
		nil, nil, nil,
		s.timeChartContainer,
	)

	// Create sub-tabs
	s.subTabs = container.NewAppTabs(
		container.NewTabItem("Win Rate", winRateContent),
		container.NewTabItem("Play Time", playTimeContent),
	)

	// Main container with controls at top and sub-tabs below
	s.container = container.NewBorder(
		controlsPanel,
		nil, nil, nil,
		s.subTabs,
	)

	s.refresh()
	return s.container
}

func (s *StatsTab) updateChartLabels() {
	bucket := "Day"
	switch s.aggregation {
	case AggregateByWeek:
		bucket = "Week"
	case AggregateByMonth:
		bucket = "Month"
	case AggregateByYear:
		bucket = "Year"
	}
	s.chartLabel.SetText(fmt.Sprintf("Net Wins/Losses by %s:", bucket))
	s.timeChartLabel.SetText(fmt.Sprintf("Play Time by %s:", bucket))
}

// Refresh reloads statistics from database
func (s *StatsTab) Refresh() {
	s.refresh()
}

func (s *StatsTab) refresh() {
	ctx := context.Background()

	// Win Rate sub-tab uses scope-selected stats. TotalGames on the struct
	// actually holds round counts when scope is Rounds — see
	// accumulateRoundOutcome.
	var (
		winRateStats *database.Stats
		winRateDaily []database.DailyStats
		err          error
	)
	if s.scope == ScopeRounds {
		winRateStats, err = database.GetRoundStats(ctx, s.db, s.currentWindow)
		if err == nil {
			winRateDaily, err = database.GetDailyRoundStats(ctx, s.db, s.currentWindow)
		}
	} else {
		winRateStats, err = database.GetStats(ctx, s.db, s.currentWindow)
		if err == nil {
			winRateDaily, err = database.GetDailyStats(ctx, s.db, s.currentWindow)
		}
	}
	if err != nil {
		s.winRateLabel.SetText("Error loading stats")
		s.totalTimeLabel.SetText("Error loading stats")
		return
	}

	unit := "games"
	countLabel := "Games"
	if s.scope == ScopeRounds {
		unit = "rounds"
		countLabel = "Rounds"
	}

	s.gamesLabel.SetText(fmt.Sprintf("%s: %d (W:%d L:%d D:%d)", countLabel, winRateStats.TotalGames, winRateStats.Wins, winRateStats.Losses, winRateStats.Draws))
	s.winRateLabel.SetText(fmt.Sprintf("Win Rate: %.1f%%", winRateStats.WinRate))
	s.ctWinRateLabel.SetText(fmt.Sprintf("CT: %.1f%% (%d/%d %s)", winRateStats.CTWinRate, winRateStats.CTWins, winRateStats.CTGames, unit))
	s.tWinRateLabel.SetText(fmt.Sprintf("T: %.1f%% (%d/%d %s)", winRateStats.TWinRate, winRateStats.TWins, winRateStats.TGames, unit))

	// Play Time is always game-scoped regardless of selected scope.
	gameStats := winRateStats
	if s.scope == ScopeRounds {
		gameStats, err = database.GetStats(ctx, s.db, s.currentWindow)
		if err != nil {
			s.totalTimeLabel.SetText("Error loading stats")
			return
		}
	}
	const minutesPerGame = 27
	totalMinutes := gameStats.TotalGames * minutesPerGame
	ctMinutes := gameStats.CTGames * minutesPerGame
	tMinutes := gameStats.TGames * minutesPerGame
	s.totalTimeLabel.SetText(fmt.Sprintf("Total Play Time: %s (%d games)", formatPlayTime(totalMinutes), gameStats.TotalGames))
	s.ctTimeLabel.SetText(fmt.Sprintf("CT: %s (%d games)", formatPlayTime(ctMinutes), gameStats.CTGames))
	s.tTimeLabel.SetText(fmt.Sprintf("T: %s (%d games)", formatPlayTime(tMinutes), gameStats.TGames))

	// Build Win Rate chart from the scope-selected daily stats.
	aggregatedWinRate := s.aggregateStats(winRateDaily)
	chart := s.buildChart(aggregatedWinRate)
	s.chartContainer.Objects = []fyne.CanvasObject{chart}
	s.chartContainer.Refresh()

	// Play Time chart always uses game-level daily stats.
	playTimeDaily, err := database.GetDailyStats(ctx, s.db, s.currentWindow)
	if err != nil {
		return
	}
	aggregatedPlayTime := s.aggregateStats(playTimeDaily)
	timeChart := s.buildTimeChart(aggregatedPlayTime)
	s.timeChartContainer.Objects = []fyne.CanvasObject{timeChart}
	s.timeChartContainer.Refresh()
}

// formatPlayTime converts minutes to a readable format (hours and minutes, or days/hours for large values)
func formatPlayTime(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}

	hours := minutes / 60
	mins := minutes % 60

	if hours < 24 {
		if mins > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}

	days := hours / 24
	remainingHours := hours % 24

	if remainingHours > 0 {
		return fmt.Sprintf("%dd %dh", days, remainingHours)
	}
	return fmt.Sprintf("%dd", days)
}

// AggregatedStats holds aggregated win/loss data for a period
type AggregatedStats struct {
	Label  string
	Wins   int
	Losses int
}

func (s *StatsTab) aggregateStats(dailyStats []database.DailyStats) []AggregatedStats {
	if len(dailyStats) == 0 {
		return nil
	}

	switch s.aggregation {
	case AggregateByWeek:
		return s.aggregateByWeek(dailyStats)
	case AggregateByMonth:
		return s.aggregateByMonth(dailyStats)
	case AggregateByYear:
		return s.aggregateByYear(dailyStats)
	default:
		return s.aggregateByDay(dailyStats)
	}
}

func (s *StatsTab) aggregateByDay(dailyStats []database.DailyStats) []AggregatedStats {
	result := make([]AggregatedStats, len(dailyStats))
	for i, ds := range dailyStats {
		result[i] = AggregatedStats{
			Label:  ds.Date.Format("01/02"),
			Wins:   ds.Wins,
			Losses: ds.Losses,
		}
	}
	return result
}

func (s *StatsTab) aggregateByWeek(dailyStats []database.DailyStats) []AggregatedStats {
	weekMap := make(map[string]*AggregatedStats)
	var weekOrder []string

	for _, ds := range dailyStats {
		year, week := ds.Date.ISOWeek()
		key := fmt.Sprintf("%d-W%02d", year, week)

		if _, exists := weekMap[key]; !exists {
			weekMap[key] = &AggregatedStats{Label: fmt.Sprintf("W%02d", week)}
			weekOrder = append(weekOrder, key)
		}
		weekMap[key].Wins += ds.Wins
		weekMap[key].Losses += ds.Losses
	}

	result := make([]AggregatedStats, len(weekOrder))
	for i, key := range weekOrder {
		result[i] = *weekMap[key]
	}
	return result
}

func (s *StatsTab) aggregateByMonth(dailyStats []database.DailyStats) []AggregatedStats {
	monthMap := make(map[string]*AggregatedStats)
	var monthOrder []string

	for _, ds := range dailyStats {
		key := ds.Date.Format("2006-01")
		label := ds.Date.Format("Jan")

		if _, exists := monthMap[key]; !exists {
			monthMap[key] = &AggregatedStats{Label: label}
			monthOrder = append(monthOrder, key)
		}
		monthMap[key].Wins += ds.Wins
		monthMap[key].Losses += ds.Losses
	}

	result := make([]AggregatedStats, len(monthOrder))
	for i, key := range monthOrder {
		result[i] = *monthMap[key]
	}
	return result
}

func (s *StatsTab) aggregateByYear(dailyStats []database.DailyStats) []AggregatedStats {
	yearMap := make(map[string]*AggregatedStats)
	var yearOrder []string

	for _, ds := range dailyStats {
		key := ds.Date.Format("2006")

		if _, exists := yearMap[key]; !exists {
			yearMap[key] = &AggregatedStats{Label: key}
			yearOrder = append(yearOrder, key)
		}
		yearMap[key].Wins += ds.Wins
		yearMap[key].Losses += ds.Losses
	}

	result := make([]AggregatedStats, len(yearOrder))
	for i, key := range yearOrder {
		result[i] = *yearMap[key]
	}
	return result
}

func (s *StatsTab) buildChart(stats []AggregatedStats) fyne.CanvasObject {
	if len(stats) == 0 {
		noDataLabel := widget.NewLabel("No data for selected period")
		noDataLabel.Alignment = fyne.TextAlignCenter
		return container.NewCenter(noDataLabel)
	}

	// Calculate net values and find max absolute value for scaling
	netValues := make([]int, len(stats))
	maxAbs := 1
	for i, st := range stats {
		netValues[i] = st.Wins - st.Losses
		abs := netValues[i]
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
		}
	}

	// Colors
	winColor := color.RGBA{R: 76, G: 175, B: 80, A: 255}  // Green
	lossColor := color.RGBA{R: 244, G: 67, B: 54, A: 255} // Red
	zeroLineColor := color.Gray{Y: 100}

	// Legend
	legendWinBox := canvas.NewRectangle(winColor)
	legendWinBox.SetMinSize(fyne.NewSize(12, 12))
	legendLossBox := canvas.NewRectangle(lossColor)
	legendLossBox.SetMinSize(fyne.NewSize(12, 12))

	legend := container.NewHBox(
		container.NewPadded(legendWinBox),
		widget.NewLabel("Net Wins"),
		widget.NewLabel("    "),
		container.NewPadded(legendLossBox),
		widget.NewLabel("Net Losses"),
	)

	// Create a custom scalable chart widget
	chart := &scalableChart{
		stats:         stats,
		netValues:     netValues,
		maxAbs:        maxAbs,
		winColor:      winColor,
		lossColor:     lossColor,
		zeroLineColor: zeroLineColor,
	}
	chart.ExtendBaseWidget(chart)

	scrollable := container.NewHScroll(chart)

	return container.NewBorder(nil, legend, nil, nil, scrollable)
}

func (s *StatsTab) buildTimeChart(stats []AggregatedStats) fyne.CanvasObject {
	if len(stats) == 0 {
		noDataLabel := widget.NewLabel("No data for selected period")
		noDataLabel.Alignment = fyne.TextAlignCenter
		return container.NewCenter(noDataLabel)
	}

	const minutesPerGame = 27

	// Calculate time values in minutes and find max value for scaling
	timeValues := make([]int, len(stats))
	maxTime := 1
	for i, st := range stats {
		totalGames := st.Wins + st.Losses
		timeValues[i] = totalGames * minutesPerGame
		if timeValues[i] > maxTime {
			maxTime = timeValues[i]
		}
	}

	// Color for time bars
	timeColor := color.RGBA{R: 33, G: 150, B: 243, A: 255} // Blue

	// Legend
	legendBox := canvas.NewRectangle(timeColor)
	legendBox.SetMinSize(fyne.NewSize(12, 12))

	legend := container.NewHBox(
		container.NewPadded(legendBox),
		widget.NewLabel("Play Time"),
	)

	// Create a custom scalable time chart widget
	chart := &scalableTimeChart{
		stats:      stats,
		timeValues: timeValues,
		maxTime:    maxTime,
		timeColor:  timeColor,
	}
	chart.ExtendBaseWidget(chart)

	scrollable := container.NewHScroll(chart)

	return container.NewBorder(nil, legend, nil, nil, scrollable)
}

// scalableChart is a custom widget that scales with available space
type scalableChart struct {
	widget.BaseWidget
	stats         []AggregatedStats
	netValues     []int
	maxAbs        int
	winColor      color.Color
	lossColor     color.Color
	zeroLineColor color.Color
}

func (c *scalableChart) CreateRenderer() fyne.WidgetRenderer {
	return &scalableChartRenderer{chart: c}
}

func (c *scalableChart) MinSize() fyne.Size {
	barWidth := float32(40)
	spacing := float32(10)
	totalWidth := float32(len(c.stats)) * (barWidth + spacing)
	if totalWidth < 300 {
		totalWidth = 300
	}
	return fyne.NewSize(totalWidth, 150)
}

type scalableChartRenderer struct {
	chart   *scalableChart
	objects []fyne.CanvasObject
}

func (r *scalableChartRenderer) Destroy() {}

func (r *scalableChartRenderer) Layout(size fyne.Size) {
	r.Refresh()
}

func (r *scalableChartRenderer) MinSize() fyne.Size {
	return r.chart.MinSize()
}

func (r *scalableChartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *scalableChartRenderer) Refresh() {
	c := r.chart
	size := c.Size()

	// Chart dimensions - scale with available height
	labelHeight := float32(15)
	chartHeight := size.Height - labelHeight
	if chartHeight < 60 {
		chartHeight = 60
	}
	halfHeight := chartHeight / 2
	barWidth := float32(40)
	spacing := float32(10)

	var bars []fyne.CanvasObject

	// Draw zero line
	totalWidth := float32(len(c.stats)) * (barWidth + spacing)
	if totalWidth < size.Width {
		totalWidth = size.Width
	}
	zeroLine := canvas.NewLine(c.zeroLineColor)
	zeroLine.Position1 = fyne.NewPos(0, halfHeight)
	zeroLine.Position2 = fyne.NewPos(totalWidth, halfHeight)
	zeroLine.StrokeWidth = 1
	bars = append(bars, zeroLine)

	for i, st := range c.stats {
		xOffset := float32(i) * (barWidth + spacing)
		net := c.netValues[i]

		// Track the bottom of the bar for label positioning
		barBottom := halfHeight // Default: at zero line

		if net != 0 {
			// Calculate bar height proportional to max value
			barHeight := float32(net) / float32(c.maxAbs) * halfHeight
			if barHeight < 0 {
				barHeight = -barHeight
			}
			// Minimum visible height
			if barHeight < 3 {
				barHeight = 3
			}

			var bar *canvas.Rectangle
			var yPos float32

			if net > 0 {
				bar = canvas.NewRectangle(c.winColor)
				yPos = halfHeight - barHeight
				barBottom = halfHeight // Bar ends at zero line
			} else {
				bar = canvas.NewRectangle(c.lossColor)
				yPos = halfHeight
				barBottom = halfHeight + barHeight // Bar ends below zero line
			}

			bar.Resize(fyne.NewSize(barWidth, barHeight))
			bar.Move(fyne.NewPos(xOffset, yPos))
			bars = append(bars, bar)

			// Net value label on bar
			labelText := fmt.Sprintf("%+d", net)
			netLabel := canvas.NewText(labelText, color.White)
			netLabel.TextSize = 10
			netLabel.Alignment = fyne.TextAlignCenter

			// Set text size to bar width and center it
			textSize := netLabel.MinSize()
			netLabel.Resize(fyne.NewSize(barWidth, textSize.Height))
			labelX := xOffset
			labelY := yPos + (barHeight-textSize.Height)/2
			netLabel.Move(fyne.NewPos(labelX, labelY))
			bars = append(bars, netLabel)
		}

		// Period label directly below the bar
		dateLabel := canvas.NewText(st.Label, color.Gray{Y: 150})
		dateLabel.TextSize = 10
		dateLabel.Move(fyne.NewPos(xOffset, barBottom+2))
		bars = append(bars, dateLabel)
	}

	r.objects = bars
}

// scalableTimeChart is a custom widget for displaying play time
type scalableTimeChart struct {
	widget.BaseWidget
	stats      []AggregatedStats
	timeValues []int
	maxTime    int
	timeColor  color.Color
}

func (c *scalableTimeChart) CreateRenderer() fyne.WidgetRenderer {
	return &scalableTimeChartRenderer{chart: c}
}

func (c *scalableTimeChart) MinSize() fyne.Size {
	barWidth := float32(40)
	spacing := float32(10)
	totalWidth := float32(len(c.stats)) * (barWidth + spacing)
	if totalWidth < 300 {
		totalWidth = 300
	}
	return fyne.NewSize(totalWidth, 150)
}

type scalableTimeChartRenderer struct {
	chart   *scalableTimeChart
	objects []fyne.CanvasObject
}

func (r *scalableTimeChartRenderer) Destroy() {}

func (r *scalableTimeChartRenderer) Layout(size fyne.Size) {
	r.Refresh()
}

func (r *scalableTimeChartRenderer) MinSize() fyne.Size {
	return r.chart.MinSize()
}

func (r *scalableTimeChartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *scalableTimeChartRenderer) Refresh() {
	c := r.chart
	size := c.Size()

	// Chart dimensions
	labelHeight := float32(15)
	chartHeight := size.Height - labelHeight
	if chartHeight < 60 {
		chartHeight = 60
	}
	barWidth := float32(40)
	spacing := float32(10)

	var bars []fyne.CanvasObject

	for i, st := range c.stats {
		xOffset := float32(i) * (barWidth + spacing)
		timeMinutes := c.timeValues[i]

		if timeMinutes > 0 {
			// Calculate bar height proportional to max value
			barHeight := float32(timeMinutes) / float32(c.maxTime) * chartHeight
			// Minimum visible height
			if barHeight < 3 {
				barHeight = 3
			}

			// Bar grows upward from bottom
			yPos := chartHeight - barHeight

			bar := canvas.NewRectangle(c.timeColor)
			bar.Resize(fyne.NewSize(barWidth, barHeight))
			bar.Move(fyne.NewPos(xOffset, yPos))
			bars = append(bars, bar)

			// Time label on bar
			labelText := formatPlayTime(timeMinutes)
			timeLabel := canvas.NewText(labelText, color.White)
			timeLabel.TextSize = 10
			timeLabel.Alignment = fyne.TextAlignCenter

			// Set text size to bar width and center it
			textSize := timeLabel.MinSize()
			timeLabel.Resize(fyne.NewSize(barWidth, textSize.Height))
			labelX := xOffset
			labelY := yPos + (barHeight-textSize.Height)/2
			timeLabel.Move(fyne.NewPos(labelX, labelY))
			bars = append(bars, timeLabel)
		}

		// Period label below the bar
		dateLabel := canvas.NewText(st.Label, color.Gray{Y: 150})
		dateLabel.TextSize = 10
		dateLabel.Move(fyne.NewPos(xOffset, chartHeight+2))
		bars = append(bars, dateLabel)
	}

	r.objects = bars
}
