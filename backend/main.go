package main

import (
	"log"
	"net/http"
	"os"

	"github.com/qubitstoai/backend/internal/db"
	"github.com/qubitstoai/backend/internal/handlers"
	"github.com/qubitstoai/backend/internal/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	database, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	mux := http.NewServeMux()
	h := handlers.New(database)

	// Health
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/v1/stats", h.Stats)

	// Public posts
	mux.HandleFunc("GET /api/v1/posts", h.ListPosts)
	mux.HandleFunc("GET /api/v1/posts/{slug}", h.GetPost)

	// Public tracks + sections
	mux.HandleFunc("GET /api/v1/tracks", h.ListTracks)
	mux.HandleFunc("GET /api/v1/tracks/{slug}", h.GetTrack)
	mux.HandleFunc("GET /api/v1/tracks/{slug}/sections", h.ListSections)
	mux.HandleFunc("GET /api/v1/tracks/{slug}/sections/{sectionSlug}/posts", h.ListPostsBySection)
	mux.HandleFunc("GET /api/v1/tracks/{slug}/lessons", h.ListLessons)
	mux.HandleFunc("GET /api/v1/tracks/{slug}/lessons/{lessonSlug}", h.GetLesson)

	// Newsletter
	mux.HandleFunc("POST /api/v1/newsletter/subscribe", h.Subscribe)

	// Admin auth (no JWT required)
	mux.HandleFunc("POST /api/v1/admin/signup", h.AdminSignup)
	mux.HandleFunc("POST /api/v1/admin/login", h.AdminLogin)

	// Admin protected routes
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /api/v1/admin/me", h.AdminMe)
	adminMux.HandleFunc("GET /api/v1/admin/posts", h.AdminListPosts)
	adminMux.HandleFunc("GET /api/v1/admin/posts/{id}", h.AdminGetPost)
	adminMux.HandleFunc("POST /api/v1/admin/posts", h.AdminCreatePost)
	adminMux.HandleFunc("PUT /api/v1/admin/posts/{id}", h.AdminUpdatePost)
	adminMux.HandleFunc("DELETE /api/v1/admin/posts/{id}", h.AdminDeletePost)
	adminMux.HandleFunc("POST /api/v1/admin/posts/{id}/publish", h.AdminPublishPost)
	adminMux.HandleFunc("POST /api/v1/admin/posts/{id}/unpublish", h.AdminUnpublishPost)

	// Mount admin mux behind JWT middleware
	mux.Handle("/api/v1/admin/me", middleware.RequireAdmin(adminMux))
	mux.Handle("/api/v1/admin/posts", middleware.RequireAdmin(adminMux))
	mux.Handle("/api/v1/admin/posts/", middleware.RequireAdmin(adminMux))

	handler := middleware.Chain(
		mux,
		middleware.Logger,
		middleware.CORS,
		middleware.RateLimit,
	)

	log.Printf("qubitstoai backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
