package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

type Handler struct{ db *sql.DB }

func New(db *sql.DB) *Handler { return &Handler{db: db} }

// ── helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// ── health ───────────────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

// ── stats ────────────────────────────────────────────────────────────────────

func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	var posts, subscribers int
	h.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE published=true`).Scan(&posts)
	h.db.QueryRow(`SELECT COUNT(*) FROM subscribers`).Scan(&subscribers)
	writeJSON(w, 200, map[string]any{
		"published_posts": posts,
		"subscribers":     subscribers,
		"tracks":          6,
	})
}

// ── posts ────────────────────────────────────────────────────────────────────

type Post struct {
	ID          int       `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content,omitempty"`
	Tag         string    `json:"tag"`
	ReadMins    int       `json:"read_mins"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *Handler) ListPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT id, slug, title, description, tag, read_mins, published, created_at
		FROM posts WHERE published=true ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Tag, &p.ReadMins, &p.Published, &p.CreatedAt)
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []Post{}
	}
	writeJSON(w, 200, posts)
}

func (h *Handler) GetPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var p Post
	err := h.db.QueryRow(`
		SELECT id, slug, title, description, content, tag, read_mins, published, created_at
		FROM posts WHERE slug=$1 AND published=true`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Content, &p.Tag, &p.ReadMins, &p.Published, &p.CreatedAt)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "post not found")
		return
	}
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	writeJSON(w, 200, p)
}

func (h *Handler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var p Post
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	p.Slug = strings.ToLower(strings.ReplaceAll(p.Title, " ", "-"))
	err := h.db.QueryRow(`
		INSERT INTO posts (slug, title, description, content, tag, read_mins)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`,
		p.Slug, p.Title, p.Description, p.Content, p.Tag, p.ReadMins).
		Scan(&p.ID, &p.CreatedAt)
	if err != nil {
		writeErr(w, 500, "could not create post")
		return
	}
	writeJSON(w, 201, p)
}

// ── tracks ───────────────────────────────────────────────────────────────────

type Track struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sort_order"`
}

func (h *Handler) ListTracks(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT id,slug,title,description,color,icon,sort_order FROM tracks ORDER BY sort_order`)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var tracks []Track
	for rows.Next() {
		var t Track
		rows.Scan(&t.ID, &t.Slug, &t.Title, &t.Description, &t.Color, &t.Icon, &t.SortOrder)
		tracks = append(tracks, t)
	}
	if tracks == nil {
		tracks = []Track{}
	}
	writeJSON(w, 200, tracks)
}

func (h *Handler) GetTrack(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var t Track
	err := h.db.QueryRow(`SELECT id,slug,title,description,color,icon,sort_order FROM tracks WHERE slug=$1`, slug).
		Scan(&t.ID, &t.Slug, &t.Title, &t.Description, &t.Color, &t.Icon, &t.SortOrder)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "track not found")
		return
	}
	writeJSON(w, 200, t)
}

// ── lessons ──────────────────────────────────────────────────────────────────

type Lesson struct {
	ID          int       `json:"id"`
	TrackSlug   string    `json:"track_slug"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content,omitempty"`
	SortOrder   int       `json:"sort_order"`
	ReadMins    int       `json:"read_mins"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
}

func (h *Handler) ListLessons(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	rows, err := h.db.Query(`
		SELECT l.id, t.slug, l.slug, l.title, l.description, l.sort_order, l.read_mins, l.published, l.created_at
		FROM lessons l JOIN tracks t ON t.id=l.track_id
		WHERE t.slug=$1 AND l.published=true ORDER BY l.sort_order`, slug)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var lessons []Lesson
	for rows.Next() {
		var l Lesson
		rows.Scan(&l.ID, &l.TrackSlug, &l.Slug, &l.Title, &l.Description, &l.SortOrder, &l.ReadMins, &l.Published, &l.CreatedAt)
		lessons = append(lessons, l)
	}
	if lessons == nil {
		lessons = []Lesson{}
	}
	writeJSON(w, 200, lessons)
}

func (h *Handler) GetLesson(w http.ResponseWriter, r *http.Request) {
	trackSlug := r.PathValue("slug")
	lessonSlug := r.PathValue("lessonSlug")
	var l Lesson
	err := h.db.QueryRow(`
		SELECT l.id, t.slug, l.slug, l.title, l.description, l.content, l.sort_order, l.read_mins, l.published, l.created_at
		FROM lessons l JOIN tracks t ON t.id=l.track_id
		WHERE t.slug=$1 AND l.slug=$2 AND l.published=true`, trackSlug, lessonSlug).
		Scan(&l.ID, &l.TrackSlug, &l.Slug, &l.Title, &l.Description, &l.Content, &l.SortOrder, &l.ReadMins, &l.Published, &l.CreatedAt)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "lesson not found")
		return
	}
	writeJSON(w, 200, l)
}

// ── newsletter ───────────────────────────────────────────────────────────────

func (h *Handler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Email == "" {
		writeErr(w, 400, "valid email required")
		return
	}
	_, err := h.db.Exec(`INSERT INTO subscribers (email) VALUES ($1) ON CONFLICT (email) DO NOTHING`, body.Email)
	if err != nil {
		writeErr(w, 500, "could not subscribe")
		return
	}
	writeJSON(w, 201, map[string]string{"message": "subscribed"})
}
