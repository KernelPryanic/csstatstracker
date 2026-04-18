package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Round represents a single round (a CT or T increment event) with its timestamp.
// game_id is NULL while a match is in progress; assigned when the game is saved.
type Round struct {
	ID        int
	GameID    sql.NullInt64
	Winner    Team
	Team      Team
	CreatedAt time.Time
}

// InsertPendingRound records a single round not yet linked to a game.
// Returns the new round ID.
func InsertPendingRound(ctx context.Context, db *sql.DB, winner, team Team) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO rounds (game_id, winner, team) VALUES (NULL, ?, ?)`,
		string(winner), string(team),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert round: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to read round id: %w", err)
	}
	return id, nil
}

// DeleteLastPendingRoundForWinner removes the most recent unassigned round where
// winner matches. Used by DecrementCT/T to undo the last scored round.
// Returns true if a row was deleted.
func DeleteLastPendingRoundForWinner(ctx context.Context, db *sql.DB, winner Team) (bool, error) {
	res, err := db.ExecContext(ctx, `
		DELETE FROM rounds
		WHERE id = (
			SELECT id FROM rounds
			WHERE game_id IS NULL AND winner = ?
			ORDER BY id DESC LIMIT 1
		)`, string(winner))
	if err != nil {
		return false, fmt.Errorf("failed to delete last pending round: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DeleteAllPendingRounds clears every unassigned round. Used by Reset.
func DeleteAllPendingRounds(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `DELETE FROM rounds WHERE game_id IS NULL`)
	if err != nil {
		return fmt.Errorf("failed to clear pending rounds: %w", err)
	}
	return nil
}

// AssignPendingRoundsToGame links every currently unassigned round to the given
// game_id. Called after SaveGame when a match completes.
func AssignPendingRoundsToGame(ctx context.Context, db *sql.DB, gameID int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE rounds SET game_id = ? WHERE game_id IS NULL`, gameID)
	if err != nil {
		return fmt.Errorf("failed to assign rounds to game: %w", err)
	}
	return nil
}

// InsertRoundForGame appends a round already linked to a specific game.
// created_at defaults to now. Used by the edit dialog when a user adds a
// missing round to an existing game.
func InsertRoundForGame(ctx context.Context, db *sql.DB, gameID int, winner, team Team) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO rounds (game_id, winner, team) VALUES (?, ?, ?)`,
		gameID, string(winner), string(team),
	)
	if err != nil {
		return fmt.Errorf("failed to insert round for game: %w", err)
	}
	return nil
}

// UpdateRoundWinner changes a round's winner.
func UpdateRoundWinner(ctx context.Context, db *sql.DB, roundID int, winner Team) error {
	_, err := db.ExecContext(ctx,
		`UPDATE rounds SET winner = ? WHERE id = ?`, string(winner), roundID)
	if err != nil {
		return fmt.Errorf("failed to update round: %w", err)
	}
	return nil
}

// DeleteRound removes a single round by id.
func DeleteRound(ctx context.Context, db *sql.DB, roundID int) error {
	_, err := db.ExecContext(ctx, `DELETE FROM rounds WHERE id = ?`, roundID)
	if err != nil {
		return fmt.Errorf("failed to delete round: %w", err)
	}
	return nil
}

// GetRoundsForGame returns all rounds belonging to a game, ordered by time.
func GetRoundsForGame(ctx context.Context, db *sql.DB, gameID int) ([]Round, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, game_id, winner, team, created_at
		 FROM rounds WHERE game_id = ? ORDER BY created_at ASC, id ASC`, gameID)
	if err != nil {
		return nil, fmt.Errorf("failed to query rounds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var rounds []Round
	for rows.Next() {
		var r Round
		var winner, team string
		if err := rows.Scan(&r.ID, &r.GameID, &winner, &team, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan round: %w", err)
		}
		r.Winner = Team(winner)
		r.Team = Team(team)
		rounds = append(rounds, r)
	}
	return rounds, rows.Err()
}
