package db

import (
	"database/sql"
	"fmt"
	"log"

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

// Migrate runs each statement independently so adding new columns/tables
// to an existing database never fails on the already-existing ones.
func Migrate(db *sql.DB) error {
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			// Log but don't fatal — some statements are expected to be no-ops
			log.Printf("migration stmt skipped/warn: %v", err)
		}
	}
	return nil
}

var migrations = []string{
	// ── Core tables ────────────────────────────────────────────────────────
	`CREATE TABLE IF NOT EXISTS tracks (
		id          SERIAL PRIMARY KEY,
		slug        TEXT UNIQUE NOT NULL,
		title       TEXT NOT NULL,
		description TEXT NOT NULL,
		color       TEXT NOT NULL DEFAULT '#7F77DD',
		icon        TEXT NOT NULL DEFAULT 'cpu',
		sort_order  INT  NOT NULL DEFAULT 0,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS sections (
		id          SERIAL PRIMARY KEY,
		track_id    INT  NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
		slug        TEXT NOT NULL,
		title       TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		sort_order  INT  NOT NULL DEFAULT 0,
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		UNIQUE(track_id, slug)
	)`,

	`CREATE TABLE IF NOT EXISTS lessons (
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
	)`,

	`CREATE TABLE IF NOT EXISTS posts (
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
	)`,

	`CREATE TABLE IF NOT EXISTS admin_users (
		id            SERIAL PRIMARY KEY,
		email         TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		name          TEXT NOT NULL DEFAULT '',
		created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	`CREATE TABLE IF NOT EXISTS subscribers (
		id         SERIAL PRIMARY KEY,
		email      TEXT UNIQUE NOT NULL,
		confirmed  BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`,

	// ── Add new columns to existing tables (safe to re-run) ────────────────
	`ALTER TABLE posts ADD COLUMN IF NOT EXISTS track_id   INT REFERENCES tracks(id) ON DELETE SET NULL`,
	`ALTER TABLE posts ADD COLUMN IF NOT EXISTS section_id INT REFERENCES sections(id) ON DELETE SET NULL`,
	`ALTER TABLE posts ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,

	// ── Indexes ────────────────────────────────────────────────────────────
	`CREATE INDEX IF NOT EXISTS idx_lessons_track_id  ON lessons(track_id)`,
	`CREATE INDEX IF NOT EXISTS idx_posts_published   ON posts(published, created_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_posts_section     ON posts(section_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sections_track_id ON sections(track_id)`,

	// ── Seed tracks ────────────────────────────────────────────────────────
	`INSERT INTO tracks (slug, title, description, color, icon, sort_order) VALUES
		('hardware',  'Hardware',           'Qubits, quantum circuits, quantum hardware, quantum computing',            '#7F77DD', 'cpu',      1),
		('systems',   'Systems',            'Assembly language, C programming, OS internals, memory management', '#1D9E75', 'terminal', 2),
		('compilers', 'Compilers & VMs',    'Lexers, parsers, bytecode, interpreters, virtual machines',         '#BA7517', 'code',     3),
		('web',       'Web & Backend',      'HTTP, databases, REST APIs, system design, frontend development',   '#185FA5', 'globe',    4),
		('ml',        'ML & Deep Learning', 'Math foundations, neural nets, transformers, LLMs',                 '#993C1D', 'brain',    5),
		('agents',    'AI Agents',          'Planning, memory, tools, autonomous systems',                       '#3B6D11', 'bot',      6)
	ON CONFLICT (slug) DO NOTHING`,

	// ── Seed Web subsections ───────────────────────────────────────────────
	`INSERT INTO sections (track_id, slug, title, description, sort_order)
		SELECT t.id, 'low-level-design',
			'Low Level Design',
			'Programming & Coding — data structures, algorithms, system internals, and hands-on implementation',
			1
		FROM tracks t WHERE t.slug = 'web'
		ON CONFLICT (track_id, slug) DO NOTHING`,

	`INSERT INTO sections (track_id, slug, title, description, sort_order)
		SELECT t.id, 'high-level-design',
			'High Level Design',
			'Design & Architecture — distributed systems, scalability, patterns, and architectural decisions',
			2
		FROM tracks t WHERE t.slug = 'web'
		ON CONFLICT (track_id, slug) DO NOTHING`,
}
