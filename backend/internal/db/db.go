package db

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

func Connect(dsn string) (*sql.DB, error) {
	if dsn == "" {
		dsn = "postgres://qubitstoai:qubitstoai@localhost:5432/qubitstoai?sslmode=disable"
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db.Ping: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	return db, nil
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE TABLE IF NOT EXISTS tracks (
    id          SERIAL PRIMARY KEY,
    slug        TEXT UNIQUE NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    color       TEXT NOT NULL DEFAULT '#7F77DD',
    icon        TEXT NOT NULL DEFAULT 'cpu',
    sort_order  INT  NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lessons (
    id          SERIAL PRIMARY KEY,
    track_id    INT  NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    slug        TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    sort_order  INT  NOT NULL DEFAULT 0,
    read_mins   INT  NOT NULL DEFAULT 8,
    published   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(track_id, slug)
);

CREATE TABLE IF NOT EXISTS posts (
    id          SERIAL PRIMARY KEY,
    slug        TEXT UNIQUE NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    tag         TEXT NOT NULL DEFAULT 'General',
    read_mins   INT  NOT NULL DEFAULT 8,
    published   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS subscribers (
    id         SERIAL PRIMARY KEY,
    email      TEXT UNIQUE NOT NULL,
    confirmed  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lessons_track_id ON lessons(track_id);
CREATE INDEX IF NOT EXISTS idx_posts_published   ON posts(published, created_at DESC);

-- Seed tracks if empty
INSERT INTO tracks (slug, title, description, color, icon, sort_order) VALUES
  ('hardware',  'Hardware',          'NAND gates, circuits, CPU architecture, memory systems', '#7F77DD', 'cpu',    1),
  ('systems',   'Systems',           'Assembly language, C programming, OS internals, memory management', '#1D9E75', 'terminal', 2),
  ('compilers', 'Compilers & VMs',   'Lexers, parsers, bytecode, interpreters, virtual machines', '#BA7517', 'code',    3),
  ('web',       'Web & Backend',     'HTTP, databases, REST APIs, frontend development', '#185FA5', 'globe',   4),
  ('ml',        'ML & Deep Learning','Math foundations, neural nets, transformers, LLMs', '#993C1D', 'brain',   5),
  ('agents',    'AI Agents',         'Planning, memory, tools, autonomous systems', '#3B6D11', 'bot',     6)
ON CONFLICT (slug) DO NOTHING;

-- Seed first lesson
INSERT INTO posts (slug, title, description, tag, read_mins, published) VALUES
  ('what-is-a-bit',
   'What is a bit? How computers think in 0s and 1s',
   'Before NAND gates, before circuits — understand why computers use binary at all.',
   'Hardware', 8, TRUE)
ON CONFLICT (slug) DO NOTHING;
`
