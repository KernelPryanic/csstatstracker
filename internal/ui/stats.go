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

// StatsTab manages the statistics view
type StatsTab struct {
	db            *sql.DB
	window        fyne.Window
	currentWindow database.TimeWindow
	aggregation   AggregationInterval
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
	totalTimeLabel  *widget.Label
	ctTimeLabel     *widget.Label
	tTimeLabel      *widget.Label
	timeChartLabel  *widget.Label
	timeChartContainer *fyne.Container
}

// NewStatsTab creates a new statistics tab
func NewStatsTab(db *sql.DB, window fyne.Window) *StatsTab {
	s := &StatsTab{
		db:            db,
		window:        window,
		currentWindow: database.WindowAll,
		aggregation:   AggregateByDay,
	}
	return s
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
			switch selected {
			case "Day":
				s.currentWindow = database.WindowDay
			case "Week":
				s.currentWindow = database.WindowWeek
			case "Month":
				s.currentWindow = database.WindowMonth
			case "Year":
				s.currentWindow = database.WindowYear
			default:
				s.currentWindow = database.WindowAll
			}
			s.refresh()
		},
	)
	windowSelect.SetSelected("All Time")

	// Aggregation selector
	aggregationSelect := widget.NewSelect(
		[]string{"By Day", "By Week", "By Month", "By Year"},
		func(selected string) {
			switch selected {
			case "By Week":
				s.aggregation = AggregateByWeek
			case "By Month":
				s.aggregation = AggregateByMonth
			case "By Year":
				s.aggregation = AggregateByYear
			default:
				s.aggregation = AggregateByDay
			}
			s.updateChartLabels()
			s.refresh()
		},
	)
	aggregationSelect.SetSelected("By Day")

	// Shared controls (Period and Group)
	controlsPanel := container.NewHBox(
		widget.NewLabel("Period:"),
		windowSelect,
		widget.NewLabel("Group:"),
		aggregationSelect,
	)

	// Win Rate sub-tab content
	winRateContent := container.NewBorder(
		container.NewVBox(
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
	switch s.aggregation {
	case AggregateByWeek:
		s.chartLabel.SetText("Net Wins/Losses by Week:")
		s.timeChartLabel.SetText("Play Time by Week:")
	case AggregateByMonth:
		s.chartLabel.SetText("Net Wins/Losses by Month:")
		s.timeChartLabel.SetText("Play Time by Month:")
	case AggregateByYear:
		s.chartLabel.SetText("Net Wins/Losses by Year:")
		s.timeChartLabel.SetText("Play Time by Year:")
	default:
		s.chartLabel.SetText("Net Wins/Losses by Day:")
		s.timeChartLabel.SetText("Play Time by Day:")
	}
}

// Refresh reloads statistics from database
func (s *StatsTab) Refresh() {
	s.refresh()
}

func (s *StatsTab) refresh() {
	ctx := context.Background()

	// Get stats
	stats, err := database.GetStats(ctx, s.db, s.currentWindow)
	if err != nil {
		s.winRateLabel.SetText("Error loading stats")
		s.totalTimeLabel.SetText("Error loading stats")
		return
	}

	// Update Win Rate labels
	s.gamesLabel.SetText(fmt.Sprintf("Games: %d (W:%d L:%d D:%d)", stats.TotalGames, stats.Wins, stats.Losses, stats.Draws))
	s.winRateLabel.SetText(fmt.Sprintf("Win Rate: %.1f%%", stats.WinRate))
	s.ctWinRateLabel.SetText(fmt.Sprintf("CT: %.1f%% (%d/%d games)", stats.CTWinRate, stats.CTWins, stats.CTGames))
	s.tWinRateLabel.SetText(fmt.Sprintf("T: %.1f%% (%d/%d games)", stats.TWinRate, stats.TWins, stats.TGames))

	// Calculate Play Time (33 minutes per game)
	const minutesPerGame = 33
	totalMinutes := stats.TotalGames * minutesPerGame
	ctMinutes := stats.CTGames * minutesPerGame
	tMinutes := stats.TGames * minutesPerGame

	// Update Play Time labels
	s.totalTimeLabel.SetText(fmt.Sprintf("Total Play Time: %s (%d games)", formatPlayTime(totalMinutes), stats.TotalGames))
	s.ctTimeLabel.SetText(fmt.Sprintf("CT: %s (%d games)", formatPlayTime(ctMinutes), stats.CTGames))
	s.tTimeLabel.SetText(fmt.Sprintf("T: %s (%d games)", formatPlayTime(tMinutes), stats.TGames))

	// Get daily stats for charts
	dailyStats, err := database.GetDailyStats(ctx, s.db, s.currentWindow)
	if err != nil {
		return
	}

	// Aggregate stats based on selected interval
	aggregatedStats := s.aggregateStats(dailyStats)

	// Build Win Rate chart
	chart := s.buildChart(aggregatedStats)
	s.chartContainer.Objects = []fyne.CanvasObject{chart}
	s.chartContainer.Refresh()

	// Build Play Time chart
	timeChart := s.buildTimeChart(aggregatedStats)
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

	const minutesPerGame = 33

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
