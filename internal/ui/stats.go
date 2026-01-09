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

	// Stat displays
	winRateLabel   *widget.Label
	ctWinRateLabel *widget.Label
	tWinRateLabel  *widget.Label
	gamesLabel     *widget.Label
	chartLabel     *widget.Label

	// Chart container
	chartContainer *fyne.Container
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
	// Stats labels (create first to avoid nil pointer in refresh)
	s.winRateLabel = widget.NewLabel("Win Rate: --")
	s.ctWinRateLabel = widget.NewLabel("CT Win Rate: --")
	s.tWinRateLabel = widget.NewLabel("T Win Rate: --")
	s.gamesLabel = widget.NewLabel("Games: 0")
	s.chartLabel = widget.NewLabel("Net Wins/Losses by Day:")

	// Chart container
	s.chartContainer = container.NewMax()

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
			s.updateChartLabel()
			s.refresh()
		},
	)
	aggregationSelect.SetSelected("By Day")

	// Stats panel
	statsPanel := container.NewVBox(
		container.NewHBox(
			widget.NewLabel("Period:"),
			windowSelect,
			widget.NewLabel("Group:"),
			aggregationSelect,
		),
		widget.NewSeparator(),
		s.gamesLabel,
		s.winRateLabel,
		widget.NewSeparator(),
		widget.NewLabel("Win Rate by Team:"),
		s.ctWinRateLabel,
		s.tWinRateLabel,
		widget.NewSeparator(),
		s.chartLabel,
	)

	s.container = container.NewBorder(
		statsPanel,
		nil, nil, nil,
		s.chartContainer,
	)

	s.refresh()
	return s.container
}

func (s *StatsTab) updateChartLabel() {
	switch s.aggregation {
	case AggregateByWeek:
		s.chartLabel.SetText("Net Wins/Losses by Week:")
	case AggregateByMonth:
		s.chartLabel.SetText("Net Wins/Losses by Month:")
	case AggregateByYear:
		s.chartLabel.SetText("Net Wins/Losses by Year:")
	default:
		s.chartLabel.SetText("Net Wins/Losses by Day:")
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
		return
	}

	// Update labels
	s.gamesLabel.SetText(fmt.Sprintf("Games: %d (W:%d L:%d D:%d)", stats.TotalGames, stats.Wins, stats.Losses, stats.Draws))
	s.winRateLabel.SetText(fmt.Sprintf("Win Rate: %.1f%%", stats.WinRate))
	s.ctWinRateLabel.SetText(fmt.Sprintf("CT: %.1f%% (%d/%d games)", stats.CTWinRate, stats.CTWins, stats.CTGames))
	s.tWinRateLabel.SetText(fmt.Sprintf("T: %.1f%% (%d/%d games)", stats.TWinRate, stats.TWins, stats.TGames))

	// Get daily stats for chart
	dailyStats, err := database.GetDailyStats(ctx, s.db, s.currentWindow)
	if err != nil {
		return
	}

	// Aggregate stats based on selected interval
	aggregatedStats := s.aggregateStats(dailyStats)

	// Build chart
	chart := s.buildChart(aggregatedStats)
	s.chartContainer.Objects = []fyne.CanvasObject{chart}
	s.chartContainer.Refresh()
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
			netLabel := canvas.NewText(fmt.Sprintf("%+d", net), color.White)
			netLabel.TextSize = 10
			netLabel.Alignment = fyne.TextAlignCenter
			labelText := fmt.Sprintf("%+d", net)
			textWidth := float32(len(labelText)) * 8
			textHeight := float32(12)
			labelX := xOffset + (barWidth-textWidth)/2
			labelY := yPos + (barHeight-textHeight)/2
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
