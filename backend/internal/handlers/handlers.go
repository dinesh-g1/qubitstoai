package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/qubitstoai/backend/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct{ db *sql.DB }

func New(db *sql.DB) *Handler { return &Handler{db: db} }

// ── helpers ───────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func toSlug(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// ── health ────────────────────────────────────────────────────────────────────

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "time": time.Now().Format(time.RFC3339)})
}

// ── stats ─────────────────────────────────────────────────────────────────────

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

// ── admin auth ────────────────────────────────────────────────────────────────

type AdminUser struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func (h *Handler) AdminSignup(w http.ResponseWriter, r *http.Request) {
	// Only allow first signup if no admins exist
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count)
	if count > 0 {
		writeErr(w, 403, "admin already exists — use login")
		return
	}

	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if body.Email == "" || len(body.Password) < 8 {
		writeErr(w, 400, "email and password (min 8 chars) required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, 500, "could not hash password")
		return
	}

	var u AdminUser
	err = h.db.QueryRow(
		`INSERT INTO admin_users (email, password_hash, name) VALUES ($1,$2,$3) RETURNING id, email, name, created_at`,
		body.Email, string(hash), body.Name,
	).Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		writeErr(w, 500, "could not create admin")
		return
	}

	token, err := auth.GenerateToken(u.ID, u.Email)
	if err != nil {
		writeErr(w, 500, "could not generate token")
		return
	}
	writeJSON(w, 201, map[string]any{"user": u, "token": token})
}

func (h *Handler) AdminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}

	var u AdminUser
	var hash string
	err := h.db.QueryRow(
		`SELECT id, email, name, password_hash, created_at FROM admin_users WHERE email=$1`,
		body.Email,
	).Scan(&u.ID, &u.Email, &u.Name, &hash, &u.CreatedAt)
	if err == sql.ErrNoRows {
		writeErr(w, 401, "invalid credentials")
		return
	}
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
		writeErr(w, 401, "invalid credentials")
		return
	}

	token, err := auth.GenerateToken(u.ID, u.Email)
	if err != nil {
		writeErr(w, 500, "could not generate token")
		return
	}
	writeJSON(w, 200, map[string]any{"user": u, "token": token})
}

func (h *Handler) AdminMe(w http.ResponseWriter, r *http.Request) {
	idStr := r.Header.Get("X-Admin-ID")
	id, _ := strconv.Atoi(idStr)
	var u AdminUser
	err := h.db.QueryRow(`SELECT id, email, name, created_at FROM admin_users WHERE id=$1`, id).
		Scan(&u.ID, &u.Email, &u.Name, &u.CreatedAt)
	if err != nil {
		writeErr(w, 404, "not found")
		return
	}
	writeJSON(w, 200, u)
}

// ── sections ──────────────────────────────────────────────────────────────────

type Section struct {
	ID          int    `json:"id"`
	TrackSlug   string `json:"track_slug"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
	PostCount   int    `json:"post_count"`
}

func (h *Handler) ListSections(w http.ResponseWriter, r *http.Request) {
	trackSlug := r.PathValue("slug")
	rows, err := h.db.Query(`
		SELECT s.id, $1, s.slug, s.title, s.description, s.sort_order,
		       COUNT(p.id) FILTER (WHERE p.published=true) AS post_count
		FROM sections s
		JOIN tracks t ON t.id = s.track_id
		LEFT JOIN posts p ON p.section_id = s.id
		WHERE t.slug = $1
		GROUP BY s.id
		ORDER BY s.sort_order`, trackSlug)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var sections []Section
	for rows.Next() {
		var s Section
		rows.Scan(&s.ID, &s.TrackSlug, &s.Slug, &s.Title, &s.Description, &s.SortOrder, &s.PostCount)
		sections = append(sections, s)
	}
	if sections == nil {
		sections = []Section{}
	}
	writeJSON(w, 200, sections)
}

// ── posts (public) ────────────────────────────────────────────────────────────

type Post struct {
	ID          int       `json:"id"`
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Content     string    `json:"content,omitempty"`
	Tag         string    `json:"tag"`
	TrackID     *int      `json:"track_id,omitempty"`
	SectionID   *int      `json:"section_id,omitempty"`
	SectionSlug string    `json:"section_slug,omitempty"`
	ReadMins    int       `json:"read_mins"`
	Published   bool      `json:"published"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (h *Handler) ListPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT p.id, p.slug, p.title, p.description, p.tag,
		       p.track_id, p.section_id, COALESCE(s.slug,''),
		       p.read_mins, p.published, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN sections s ON s.id = p.section_id
		WHERE p.published=true ORDER BY p.created_at DESC LIMIT 20`)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Tag,
			&p.TrackID, &p.SectionID, &p.SectionSlug,
			&p.ReadMins, &p.Published, &p.CreatedAt, &p.UpdatedAt)
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []Post{}
	}
	writeJSON(w, 200, posts)
}

func (h *Handler) ListPostsBySection(w http.ResponseWriter, r *http.Request) {
	trackSlug := r.PathValue("slug")
	sectionSlug := r.PathValue("sectionSlug")
	rows, err := h.db.Query(`
		SELECT p.id, p.slug, p.title, p.description, p.tag,
		       p.track_id, p.section_id, COALESCE(s.slug,''),
		       p.read_mins, p.published, p.created_at, p.updated_at
		FROM posts p
		JOIN sections s  ON s.id  = p.section_id
		JOIN tracks   t  ON t.id  = s.track_id
		WHERE t.slug=$1 AND s.slug=$2 AND p.published=true
		ORDER BY p.created_at DESC`, trackSlug, sectionSlug)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Tag,
			&p.TrackID, &p.SectionID, &p.SectionSlug,
			&p.ReadMins, &p.Published, &p.CreatedAt, &p.UpdatedAt)
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
		SELECT p.id, p.slug, p.title, p.description, p.content, p.tag,
		       p.track_id, p.section_id, COALESCE(s.slug,''),
		       p.read_mins, p.published, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN sections s ON s.id = p.section_id
		WHERE p.slug=$1 AND p.published=true`, slug).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Content, &p.Tag,
			&p.TrackID, &p.SectionID, &p.SectionSlug,
			&p.ReadMins, &p.Published, &p.CreatedAt, &p.UpdatedAt)
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

// ── posts (admin) ─────────────────────────────────────────────────────────────

func (h *Handler) AdminListPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`
		SELECT p.id, p.slug, p.title, p.description, p.tag,
		       p.track_id, p.section_id, COALESCE(s.slug,''),
		       p.read_mins, p.published, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN sections s ON s.id = p.section_id
		ORDER BY p.created_at DESC`)
	if err != nil {
		writeErr(w, 500, "database error")
		return
	}
	defer rows.Close()
	var posts []Post
	for rows.Next() {
		var p Post
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Tag,
			&p.TrackID, &p.SectionID, &p.SectionSlug,
			&p.ReadMins, &p.Published, &p.CreatedAt, &p.UpdatedAt)
		posts = append(posts, p)
	}
	if posts == nil {
		posts = []Post{}
	}
	writeJSON(w, 200, posts)
}

func (h *Handler) AdminGetPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var p Post
	err := h.db.QueryRow(`
		SELECT p.id, p.slug, p.title, p.description, p.content, p.tag,
		       p.track_id, p.section_id, COALESCE(s.slug,''),
		       p.read_mins, p.published, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN sections s ON s.id = p.section_id
		WHERE p.id=$1`, id).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Content, &p.Tag,
			&p.TrackID, &p.SectionID, &p.SectionSlug,
			&p.ReadMins, &p.Published, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		writeErr(w, 404, "post not found")
		return
	}
	writeJSON(w, 200, p)
}

func (h *Handler) AdminCreatePost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Tag         string `json:"tag"`
		SectionID   *int   `json:"section_id"`
		TrackID     *int   `json:"track_id"`
		ReadMins    int    `json:"read_mins"`
		Published   bool   `json:"published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if body.Title == "" {
		writeErr(w, 400, "title required")
		return
	}
	if body.ReadMins == 0 {
		body.ReadMins = 5
	}
	if body.Tag == "" {
		body.Tag = "General"
	}
	slug := toSlug(body.Title)

	var p Post
	err := h.db.QueryRow(`
		INSERT INTO posts (slug, title, description, content, tag, track_id, section_id, read_mins, published)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, slug, title, description, tag, track_id, section_id, read_mins, published, created_at, updated_at`,
		slug, body.Title, body.Description, body.Content, body.Tag,
		body.TrackID, body.SectionID, body.ReadMins, body.Published).
		Scan(&p.ID, &p.Slug, &p.Title, &p.Description, &p.Tag,
			&p.TrackID, &p.SectionID,
			&p.ReadMins, &p.Published, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		writeErr(w, 500, "could not create post: "+err.Error())
		return
	}
	writeJSON(w, 201, p)
}

func (h *Handler) AdminUpdatePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Content     string `json:"content"`
		Tag         string `json:"tag"`
		SectionID   *int   `json:"section_id"`
		TrackID     *int   `json:"track_id"`
		ReadMins    int    `json:"read_mins"`
		Published   bool   `json:"published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	_, err := h.db.Exec(`
		UPDATE posts SET
		  title=$1, description=$2, content=$3, tag=$4,
		  track_id=$5, section_id=$6, read_mins=$7, published=$8,
		  updated_at=NOW()
		WHERE id=$9`,
		body.Title, body.Description, body.Content, body.Tag,
		body.TrackID, body.SectionID, body.ReadMins, body.Published, id)
	if err != nil {
		writeErr(w, 500, "could not update post")
		return
	}
	writeJSON(w, 200, map[string]string{"message": "updated"})
}

func (h *Handler) AdminDeletePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	_, err := h.db.Exec(`DELETE FROM posts WHERE id=$1`, id)
	if err != nil {
		writeErr(w, 500, "could not delete post")
		return
	}
	writeJSON(w, 200, map[string]string{"message": "deleted"})
}

func (h *Handler) AdminPublishPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	_, err := h.db.Exec(`UPDATE posts SET published=true, updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeErr(w, 500, "could not publish post")
		return
	}
	writeJSON(w, 200, map[string]string{"message": "published"})
}

func (h *Handler) AdminUnpublishPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	_, err := h.db.Exec(`UPDATE posts SET published=false, updated_at=NOW() WHERE id=$1`, id)
	if err != nil {
		writeErr(w, 500, "could not unpublish post")
		return
	}
	writeJSON(w, 200, map[string]string{"message": "unpublished"})
}

// ── tracks ────────────────────────────────────────────────────────────────────

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

// ── lessons ───────────────────────────────────────────────────────────────────

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

// ── newsletter ────────────────────────────────────────────────────────────────

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
