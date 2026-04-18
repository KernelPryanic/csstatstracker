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

// SaveGame stores a game result in the database and returns its new ID.
func SaveGame(ctx context.Context, db *sql.DB, ctScore, tScore, gameScore int, team Team) (int64, error) {
	query := `INSERT INTO game_stats (ct_score, t_score, game_score, team) VALUES (?, ?, ?, ?)`
	res, err := db.ExecContext(ctx, query, ctScore, tScore, gameScore, string(team))
	if err != nil {
		return 0, fmt.Errorf("failed to save game stats: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to read game id: %w", err)
	}
	return id, nil
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

// GetRoundStats returns round-scope aggregate statistics for the given window.
//
// For games that have real round rows, those are counted directly. For games
// without round rows (pre-feature history, or externally edited games), rounds
// are derived from ct_score + t_score: the player's team contributes that many
// wins, the opposing team that many losses. This keeps totals backward
// compatible.
func GetRoundStats(ctx context.Context, db *sql.DB, window TimeWindow) (*Stats, error) {
	stats := &Stats{}

	startTime := GetWindowStart(window)
	useWindow := window != WindowAll

	// 1) Real rounds attached to games in the window. Only the player's team
	// for that game matters (a CT round is a win if the player was CT, a loss
	// if T, ignored if None).
	var roundRows *sql.Rows
	var err error
	if useWindow {
		roundRows, err = db.QueryContext(ctx, `
			SELECT r.winner, g.team
			FROM rounds r
			JOIN game_stats g ON g.id = r.game_id
			WHERE g.created_at >= ?`, startTime)
	} else {
		roundRows, err = db.QueryContext(ctx, `
			SELECT r.winner, g.team
			FROM rounds r
			JOIN game_stats g ON g.id = r.game_id`)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query real rounds: %w", err)
	}
	defer func() { _ = roundRows.Close() }()

	for roundRows.Next() {
		var winnerStr, teamStr string
		if err := roundRows.Scan(&winnerStr, &teamStr); err != nil {
			return nil, fmt.Errorf("failed to scan round: %w", err)
		}
		accumulateRoundOutcome(stats, Team(winnerStr), Team(teamStr))
	}
	if err := roundRows.Err(); err != nil {
		return nil, err
	}

	// 2) Games without rounds — derive from final scores.
	var legacyRows *sql.Rows
	legacyQuery := `
		SELECT ct_score, t_score, team
		FROM game_stats g
		WHERE NOT EXISTS (SELECT 1 FROM rounds r WHERE r.game_id = g.id)`
	if useWindow {
		legacyRows, err = db.QueryContext(ctx, legacyQuery+` AND created_at >= ?`, startTime)
	} else {
		legacyRows, err = db.QueryContext(ctx, legacyQuery)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query legacy games: %w", err)
	}
	defer func() { _ = legacyRows.Close() }()

	for legacyRows.Next() {
		var ctScore, tScore int
		var teamStr string
		if err := legacyRows.Scan(&ctScore, &tScore, &teamStr); err != nil {
			return nil, fmt.Errorf("failed to scan legacy game: %w", err)
		}
		team := Team(teamStr)
		for i := 0; i < ctScore; i++ {
			accumulateRoundOutcome(stats, TeamCT, team)
		}
		for i := 0; i < tScore; i++ {
			accumulateRoundOutcome(stats, TeamT, team)
		}
	}
	if err := legacyRows.Err(); err != nil {
		return nil, err
	}

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

// accumulateRoundOutcome folds a single round (winner + player's team) into
// the Stats struct. TotalGames/CTGames/TGames are reused as round counters
// when Stats is built by GetRoundStats.
func accumulateRoundOutcome(stats *Stats, winner, playerTeam Team) {
	stats.TotalGames++
	switch playerTeam {
	case TeamCT:
		stats.CTGames++
		if winner == TeamCT {
			stats.Wins++
			stats.CTWins++
		} else {
			stats.Losses++
			stats.CTLosses++
		}
	case TeamT:
		stats.TGames++
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

// GetDailyRoundStats returns daily win/loss counts in round scope.
func GetDailyRoundStats(ctx context.Context, db *sql.DB, window TimeWindow) ([]DailyStats, error) {
	startTime := GetWindowStart(window)
	useWindow := window != WindowAll

	dailyMap := make(map[string]*DailyStats)

	addRound := func(day string, winner, playerTeam Team) {
		if _, ok := dailyMap[day]; !ok {
			d, _ := time.Parse("2006-01-02", day)
			dailyMap[day] = &DailyStats{Date: d}
		}
		ds := dailyMap[day]
		switch playerTeam {
		case TeamCT:
			if winner == TeamCT {
				ds.Wins++
			} else {
				ds.Losses++
			}
		case TeamT:
			if winner == TeamT {
				ds.Wins++
			} else {
				ds.Losses++
			}
		default:
			ds.Draws++
		}
	}

	// Real rounds — use game date so aggregation matches game-scope charts.
	var rows *sql.Rows
	var err error
	if useWindow {
		rows, err = db.QueryContext(ctx, `
			SELECT date(g.created_at), r.winner, g.team
			FROM rounds r JOIN game_stats g ON g.id = r.game_id
			WHERE g.created_at >= ?`, startTime)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT date(g.created_at), r.winner, g.team
			FROM rounds r JOIN game_stats g ON g.id = r.game_id`)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query daily rounds: %w", err)
	}
	for rows.Next() {
		var day, winner, team string
		if err := rows.Scan(&day, &winner, &team); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan daily round: %w", err)
		}
		addRound(day, Team(winner), Team(team))
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("failed iterating daily rounds: %w", err)
	}
	_ = rows.Close()

	// Legacy games without rounds.
	legacyQuery := `
		SELECT date(created_at), ct_score, t_score, team
		FROM game_stats g
		WHERE NOT EXISTS (SELECT 1 FROM rounds r WHERE r.game_id = g.id)`
	if useWindow {
		rows, err = db.QueryContext(ctx, legacyQuery+` AND created_at >= ?`, startTime)
	} else {
		rows, err = db.QueryContext(ctx, legacyQuery)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query legacy daily: %w", err)
	}
	for rows.Next() {
		var day, team string
		var ct, t int
		if err := rows.Scan(&day, &ct, &t, &team); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("failed to scan legacy daily: %w", err)
		}
		playerTeam := Team(team)
		for i := 0; i < ct; i++ {
			addRound(day, TeamCT, playerTeam)
		}
		for i := 0; i < t; i++ {
			addRound(day, TeamT, playerTeam)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("failed iterating legacy daily: %w", err)
	}
	_ = rows.Close()

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
