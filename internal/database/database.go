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

// Team represents which team the player was on
type Team string

const (
	TeamNone Team = ""
	TeamCT   Team = "CT"
	TeamT    Team = "T"
)

// GameStats represents a game record from the database
type GameStats struct {
	ID        int
	CTScore   int
	TScore    int
	GameScore int
	Team      Team
	CreatedAt time.Time
}

const DefaultDBFile = "./csstatstracker.db"

// Init opens the database and runs migrations using embedded files
func Init(ctx context.Context, dbPath string, migrationsFS embed.FS) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create migration source from embedded FS
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to create migration source: %w", err)
	}

	// Run migrations
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

// SaveGame stores a game result in the database
func SaveGame(ctx context.Context, db *sql.DB, ctScore, tScore, gameScore int, team Team) error {
	query := `INSERT INTO game_stats (ct_score, t_score, game_score, team) VALUES (?, ?, ?, ?)`
	_, err := db.ExecContext(ctx, query, ctScore, tScore, gameScore, string(team))
	if err != nil {
		return fmt.Errorf("failed to save game stats: %w", err)
	}
	return nil
}

// GetAllGames returns all game records ordered by date descending
func GetAllGames(ctx context.Context, db *sql.DB) ([]GameStats, error) {
	query := `SELECT id, ct_score, t_score, game_score, team, created_at FROM game_stats ORDER BY created_at DESC`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query games: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var games []GameStats
	for rows.Next() {
		var g GameStats
		var team string
		if err := rows.Scan(&g.ID, &g.CTScore, &g.TScore, &g.GameScore, &team, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan game: %w", err)
		}
		g.Team = Team(team)
		games = append(games, g)
	}
	return games, rows.Err()
}

// UpdateGame updates an existing game record
func UpdateGame(ctx context.Context, db *sql.DB, id, ctScore, tScore, gameScore int, team Team) error {
	query := `UPDATE game_stats SET ct_score = ?, t_score = ?, game_score = ?, team = ? WHERE id = ?`
	_, err := db.ExecContext(ctx, query, ctScore, tScore, gameScore, string(team), id)
	if err != nil {
		return fmt.Errorf("failed to update game: %w", err)
	}
	return nil
}

// DeleteGame removes a game record
func DeleteGame(ctx context.Context, db *sql.DB, id int) error {
	query := `DELETE FROM game_stats WHERE id = ?`
	_, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete game: %w", err)
	}
	return nil
}

// TimeWindow represents a time period for filtering statistics
type TimeWindow int

const (
	WindowDay TimeWindow = iota
	WindowWeek
	WindowMonth
	WindowYear
	WindowAll
)

// GetWindowStart returns the start time for the given window
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
		return time.Time{} // Zero time for all
	}
}

// Stats holds aggregated statistics
type Stats struct {
	TotalGames int
	Wins       int
	Losses     int
	Draws      int
	WinRate    float64
	CTWins     int
	CTLosses   int
	CTGames    int
	CTWinRate  float64
	TWins      int
	TLosses    int
	TGames     int
	TWinRate   float64
}

// DailyStats holds win/loss counts for a specific date
type DailyStats struct {
	Date   time.Time
	Wins   int
	Losses int
	Draws  int
}

// GetStats returns aggregated statistics for the given time window
func GetStats(ctx context.Context, db *sql.DB, window TimeWindow) (*Stats, error) {
	startTime := GetWindowStart(window)

	var query string
	var rows *sql.Rows
	var err error

	if window == WindowAll {
		query = `SELECT ct_score, t_score, game_score, team FROM game_stats`
		rows, err = db.QueryContext(ctx, query)
	} else {
		query = `SELECT ct_score, t_score, game_score, team FROM game_stats WHERE created_at >= ?`
		rows, err = db.QueryContext(ctx, query, startTime)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	stats := &Stats{}

	for rows.Next() {
		var ctScore, tScore, gameScore int
		var team string
		if err := rows.Scan(&ctScore, &tScore, &gameScore, &team); err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}

		stats.TotalGames++

		// Determine if player won based on their team
		playerTeam := Team(team)
		var playerWon, playerLost bool

		switch playerTeam {
		case TeamCT:
			stats.CTGames++
			if ctScore > tScore {
				playerWon = true
				stats.CTWins++
			} else if tScore > ctScore {
				playerLost = true
				stats.CTLosses++
			}
		case TeamT:
			stats.TGames++
			if tScore > ctScore {
				playerWon = true
				stats.TWins++
			} else if ctScore > tScore {
				playerLost = true
				stats.TLosses++
			}
		default:
			// No team selected - can't determine win/loss
			stats.Draws++
			continue
		}

		if playerWon {
			stats.Wins++
		} else if playerLost {
			stats.Losses++
		} else {
			stats.Draws++
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Calculate win rates
	if stats.TotalGames > 0 {
		stats.WinRate = float64(stats.Wins) / float64(stats.TotalGames) * 100
	}
	if stats.CTGames > 0 {
		stats.CTWinRate = float64(stats.CTWins) / float64(stats.CTGames) * 100
	}
	if stats.TGames > 0 {
		stats.TWinRate = float64(stats.TWins) / float64(stats.TGames) * 100
	}

	return stats, nil
}

// GetDailyStats returns daily win/loss statistics for the given time window
func GetDailyStats(ctx context.Context, db *sql.DB, window TimeWindow) ([]DailyStats, error) {
	startTime := GetWindowStart(window)

	var query string
	var rows *sql.Rows
	var err error

	if window == WindowAll {
		query = `SELECT date(created_at) as day, ct_score, t_score, team
			FROM game_stats
			ORDER BY day ASC`
		rows, err = db.QueryContext(ctx, query)
	} else {
		query = `SELECT date(created_at) as day, ct_score, t_score, team
			FROM game_stats
			WHERE created_at >= ?
			ORDER BY day ASC`
		rows, err = db.QueryContext(ctx, query, startTime)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query daily stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Map to accumulate stats by date
	dailyMap := make(map[string]*DailyStats)

	for rows.Next() {
		var dayStr string
		var ctScore, tScore int
		var team string
		if err := rows.Scan(&dayStr, &ctScore, &tScore, &team); err != nil {
			return nil, fmt.Errorf("failed to scan daily stats: %w", err)
		}

		if _, exists := dailyMap[dayStr]; !exists {
			day, _ := time.Parse("2006-01-02", dayStr)
			dailyMap[dayStr] = &DailyStats{Date: day}
		}

		ds := dailyMap[dayStr]
		playerTeam := Team(team)

		switch playerTeam {
		case TeamCT:
			if ctScore > tScore {
				ds.Wins++
			} else if tScore > ctScore {
				ds.Losses++
			} else {
				ds.Draws++
			}
		case TeamT:
			if tScore > ctScore {
				ds.Wins++
			} else if ctScore > tScore {
				ds.Losses++
			} else {
				ds.Draws++
			}
		default:
			ds.Draws++
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert map to sorted slice
	var result []DailyStats
	for _, ds := range dailyMap {
		result = append(result, *ds)
	}

	// Sort by date
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Date.After(result[j].Date) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result, nil
}
