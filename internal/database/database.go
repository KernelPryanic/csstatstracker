package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

// Team represents which team the player was on when a round was recorded.
type Team string

const (
	TeamNone Team = ""
	TeamCT   Team = "CT"
	TeamT    Team = "T"
)

const DefaultDBFile = "./csstatstracker.db"

// Init opens the database and runs migrations using embedded files.
func Init(ctx context.Context, dbPath string, migrationsFS embed.FS) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}

	driver, err := sqlite.WithInstance(db, &sqlite.Config{})
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// TimeWindow represents a time period for filtering statistics.
type TimeWindow int

const (
	WindowDay TimeWindow = iota
	WindowWeek
	WindowMonth
	WindowYear
	WindowAll
)

// GetWindowStart returns the start time for the given window.
func GetWindowStart(window TimeWindow) time.Time {
	now := time.Now()
	switch window {
	case WindowDay:
		return now.AddDate(0, 0, -1)
	case WindowWeek:
		return now.AddDate(0, 0, -7)
	case WindowMonth:
		return now.AddDate(0, -1, 0)
	case WindowYear:
		return now.AddDate(-1, 0, 0)
	default:
		return time.Time{}
	}
}

// Stats holds aggregate round counts for a window.
type Stats struct {
	TotalRounds int
	Wins        int
	Losses      int
	Draws       int // rounds played with no team selected
	WinRate     float64
	CTRounds    int
	CTWins      int
	CTLosses    int
	CTWinRate   float64
	TRounds     int
	TWins       int
	TLosses     int
	TWinRate    float64
}

// DailyStats holds win/loss counts for a specific date.
type DailyStats struct {
	Date   time.Time
	Wins   int
	Losses int
	Draws  int
}

// GetStats returns round-scope aggregate statistics for the given window.
func GetStats(ctx context.Context, db *sql.DB, window TimeWindow) (*Stats, error) {
	startTime := GetWindowStart(window)
	useWindow := window != WindowAll

	var rows *sql.Rows
	var err error
	if useWindow {
		rows, err = db.QueryContext(ctx,
			`SELECT winner, team FROM rounds WHERE created_at >= ?`, startTime)
	} else {
		rows, err = db.QueryContext(ctx, `SELECT winner, team FROM rounds`)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	stats := &Stats{}
	for rows.Next() {
		var winner, team string
		if err := rows.Scan(&winner, &team); err != nil {
			return nil, fmt.Errorf("failed to scan round: %w", err)
		}
		accumulate(stats, Team(winner), Team(team))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if stats.TotalRounds > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalRounds) * 100
	}
	if stats.CTRounds > 0 {
		stats.CTWinRate = float64(stats.CTWins) / float64(stats.CTRounds) * 100
	}
	if stats.TRounds > 0 {
		stats.TWinRate = float64(stats.TWins) / float64(stats.TRounds) * 100
	}
	return stats, nil
}

func accumulate(stats *Stats, winner, playerTeam Team) {
	stats.TotalRounds++
	switch playerTeam {
	case TeamCT:
		stats.CTRounds++
		if winner == TeamCT {
			stats.Wins++
			stats.CTWins++
		} else {
			stats.Losses++
			stats.CTLosses++
		}
	case TeamT:
		stats.TRounds++
		if winner == TeamT {
			stats.Wins++
			stats.TWins++
		} else {
			stats.Losses++
			stats.TLosses++
		}
	default:
		stats.Draws++
	}
}

// GetDailyStats returns daily win/loss counts (round-scope).
func GetDailyStats(ctx context.Context, db *sql.DB, window TimeWindow) ([]DailyStats, error) {
	startTime := GetWindowStart(window)
	useWindow := window != WindowAll

	var rows *sql.Rows
	var err error
	if useWindow {
		rows, err = db.QueryContext(ctx, `
			SELECT date(created_at), winner, team
			FROM rounds
			WHERE created_at >= ?
			ORDER BY created_at ASC`, startTime)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT date(created_at), winner, team
			FROM rounds
			ORDER BY created_at ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query daily stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	dailyMap := make(map[string]*DailyStats)
	for rows.Next() {
		var day, winner, team string
		if err := rows.Scan(&day, &winner, &team); err != nil {
			return nil, fmt.Errorf("failed to scan daily row: %w", err)
		}
		if _, ok := dailyMap[day]; !ok {
			d, _ := time.Parse("2006-01-02", day)
			dailyMap[day] = &DailyStats{Date: d}
		}
		ds := dailyMap[day]
		playerTeam := Team(team)
		switch playerTeam {
		case TeamCT:
			if winner == string(TeamCT) {
				ds.Wins++
			} else {
				ds.Losses++
			}
		case TeamT:
			if winner == string(TeamT) {
				ds.Wins++
			} else {
				ds.Losses++
			}
		default:
			ds.Draws++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	result := make([]DailyStats, 0, len(dailyMap))
	for _, ds := range dailyMap {
		result = append(result, *ds)
	}
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Date.After(result[j].Date) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result, nil
}
