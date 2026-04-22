-- Down migration is best-effort: we can recreate the game_stats table, but we
-- cannot reconstitute the mapping from rounds back to games. This leaves
-- game_stats empty and re-attaches an optional game_id column on rounds so
-- the previous schema shape is restored.

CREATE TABLE IF NOT EXISTS game_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ct_score INTEGER NOT NULL,
    t_score INTEGER NOT NULL,
    game_score INTEGER NOT NULL,
    team TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_created_at ON game_stats(created_at);

CREATE TABLE rounds_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER,
    winner TEXT NOT NULL,
    team TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (game_id) REFERENCES game_stats(id) ON DELETE CASCADE
);

INSERT INTO rounds_new (id, game_id, winner, team, created_at)
SELECT id, NULL, winner, team, created_at FROM rounds;

DROP INDEX IF EXISTS idx_rounds_created_at;
DROP TABLE rounds;
ALTER TABLE rounds_new RENAME TO rounds;

CREATE INDEX IF NOT EXISTS idx_rounds_game_id ON rounds(game_id);
CREATE INDEX IF NOT EXISTS idx_rounds_created_at ON rounds(created_at);
