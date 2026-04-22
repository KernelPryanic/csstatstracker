package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Round represents a single round recorded by the tracker.
type Round struct {
	ID        int
	Winner    Team
	Team      Team
	CreatedAt time.Time
}

// InsertRound records a round with the given winner and player's team.
// Returns the new row id.
func InsertRound(ctx context.Context, db *sql.DB, winner, team Team) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO rounds (winner, team) VALUES (?, ?)`,
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

// DeleteLastRoundForWinner removes the most recent round whose winner matches,
// used by the tracker's decrement buttons.
func DeleteLastRoundForWinner(ctx context.Context, db *sql.DB, winner Team) (bool, error) {
	res, err := db.ExecContext(ctx, `
		DELETE FROM rounds
		WHERE id = (
			SELECT id FROM rounds
			WHERE winner = ?
			ORDER BY id DESC LIMIT 1
		)`, string(winner))
	if err != nil {
		return false, fmt.Errorf("failed to delete last round: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// UpdateRound mutates a round's winner and/or team.
func UpdateRound(ctx context.Context, db *sql.DB, id int, winner, team Team) error {
	_, err := db.ExecContext(ctx,
		`UPDATE rounds SET winner = ?, team = ? WHERE id = ?`,
		string(winner), string(team), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update round: %w", err)
	}
	return nil
}

// DeleteRound removes a single round by id.
func DeleteRound(ctx context.Context, db *sql.DB, id int) error {
	_, err := db.ExecContext(ctx, `DELETE FROM rounds WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete round: %w", err)
	}
	return nil
}

// GetAllRounds returns every round in reverse-chronological order.
func GetAllRounds(ctx context.Context, db *sql.DB) ([]Round, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, winner, team, created_at FROM rounds ORDER BY created_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("failed to query rounds: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []Round
	for rows.Next() {
		var r Round
		var winner, team string
		if err := rows.Scan(&r.ID, &winner, &team, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan round: %w", err)
		}
		r.Winner = Team(winner)
		r.Team = Team(team)
		out = append(out, r)
	}
	return out, rows.Err()
}
