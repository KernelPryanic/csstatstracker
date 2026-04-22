-- Migration to rounds-only schema.
--
-- For every existing game that does NOT already have round rows, synthesise
-- one round per CT score and one per T score, all timestamped with the game's
-- created_at (we have no finer-grained data for legacy games). The player's
-- team from game_stats becomes the `team` column on each synthetic round.
--
-- After synthesis, rounds are detached from games (game_id set NULL) and the
-- games table is dropped entirely.

-- 1) Synthesise CT rounds for legacy games.
WITH RECURSIVE
    legacy_games AS (
        SELECT g.id, g.ct_score, g.t_score, g.team, g.created_at
        FROM game_stats g
        WHERE NOT EXISTS (SELECT 1 FROM rounds r WHERE r.game_id = g.id)
    ),
    ct_counter(game_id, n, ct_score, team, created_at) AS (
        SELECT id, 1, ct_score, team, created_at FROM legacy_games WHERE ct_score > 0
        UNION ALL
        SELECT game_id, n + 1, ct_score, team, created_at
        FROM ct_counter
        WHERE n < ct_score
    )
INSERT INTO rounds (game_id, winner, team, created_at)
SELECT game_id, 'CT', team, created_at FROM ct_counter;

-- 2) Synthesise T rounds for legacy games.
WITH RECURSIVE
    legacy_games AS (
        SELECT g.id, g.ct_score, g.t_score, g.team, g.created_at
        FROM game_stats g
        WHERE NOT EXISTS (
            SELECT 1 FROM rounds r
            WHERE r.game_id = g.id AND r.winner = 'T'
        ) AND g.t_score > 0
    ),
    t_counter(game_id, n, t_score, team, created_at) AS (
        SELECT id, 1, t_score, team, created_at FROM legacy_games
        UNION ALL
        SELECT game_id, n + 1, t_score, team, created_at
        FROM t_counter
        WHERE n < t_score
    )
INSERT INTO rounds (game_id, winner, team, created_at)
SELECT game_id, 'T', team, created_at FROM t_counter;

-- 3) Detach rounds from games; game_id is no longer meaningful.
UPDATE rounds SET game_id = NULL;

-- 4) Drop the games table and its index.
DROP INDEX IF EXISTS idx_created_at;
DROP TABLE IF EXISTS game_stats;

-- 5) The game_id column on rounds is now always NULL. SQLite cannot drop a
-- column with a foreign key pre-3.35, so leave it; recreate the table without
-- it for cleanliness.
CREATE TABLE rounds_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    winner TEXT NOT NULL,
    team TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO rounds_new (id, winner, team, created_at)
SELECT id, winner, team, created_at FROM rounds;

DROP INDEX IF EXISTS idx_rounds_created_at;
DROP INDEX IF EXISTS idx_rounds_game_id;
DROP TABLE rounds;
ALTER TABLE rounds_new RENAME TO rounds;

CREATE INDEX IF NOT EXISTS idx_rounds_created_at ON rounds(created_at);
