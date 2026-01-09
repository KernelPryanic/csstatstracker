CREATE TABLE IF NOT EXISTS game_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ct_score INTEGER NOT NULL,
    t_score INTEGER NOT NULL,
    game_score INTEGER NOT NULL,
    team TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_created_at ON game_stats(created_at);
