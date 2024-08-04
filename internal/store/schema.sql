PRAGMA journal_mode=WAL;

CREATE TABLE IF NOT EXISTS tx_log (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL,
  from_addr TEXT NOT NULL,
  to_addr TEXT NOT NULL,
  amount INTEGER NOT NULL,
  status TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS watchlist (
  chat_id INTEGER NOT NULL,
  addr TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  PRIMARY KEY(chat_id, addr)
);
