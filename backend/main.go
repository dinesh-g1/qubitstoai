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

	// Initialize database
	database, err := db.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	// Build router
	mux := http.NewServeMux()
	h := handlers.New(database)

	// Health check
	mux.HandleFunc("GET /health", h.Health)

	// API v1
	mux.HandleFunc("GET /api/v1/posts", h.ListPosts)
	mux.HandleFunc("GET /api/v1/posts/{slug}", h.GetPost)
	mux.HandleFunc("POST /api/v1/posts", h.CreatePost)

	mux.HandleFunc("GET /api/v1/tracks", h.ListTracks)
	mux.HandleFunc("GET /api/v1/tracks/{slug}", h.GetTrack)
	mux.HandleFunc("GET /api/v1/tracks/{slug}/lessons", h.ListLessons)
	mux.HandleFunc("GET /api/v1/tracks/{slug}/lessons/{lessonSlug}", h.GetLesson)

	mux.HandleFunc("POST /api/v1/newsletter/subscribe", h.Subscribe)

	mux.HandleFunc("GET /api/v1/stats", h.Stats)

	// Wrap with middleware
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
