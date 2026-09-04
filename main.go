package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"askworx-whatsapp-bot/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/joho/godotenv"
)

// requireEnv collects everything the process cannot run safely without.
//
// The old build had silent fallbacks: a missing API_SECRET became the
// hardcoded string "dummy-token-askworx", and a missing ADMIN_PASSWORD meant
// an empty password was accepted. Both failures were invisible — the service
// started, looked healthy, and was open. Anything missing now stops the boot
// with a message naming the variable.
func requireEnv(names ...string) error {
	var missing []string
	for _, n := range names {
		if strings.TrimSpace(os.Getenv(n)) == "" {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required environment variables are not set: %s", strings.Join(missing, ", "))
	}
	return nil
}

// allowedOrigins reads CORS_ORIGINS as a comma-separated list.
//
// It was ["*"] together with AllowCredentials — a combination browsers reject
// outright, and the wrong posture for an admin API regardless. In development
// an unset value falls back to the local panel; in production it is required,
// because "*" on the console is not a default anybody should inherit silently.
func allowedOrigins() ([]string, error) {
	raw := strings.TrimSpace(os.Getenv("CORS_ORIGINS"))
	if raw == "" {
		if isProduction() {
			return nil, errors.New("CORS_ORIGINS must list the admin panel's origin in production")
		}
		return []string{"http://localhost:5173", "http://127.0.0.1:5173"}, nil
	}

	var origins []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			if o == "*" {
				return nil, errors.New(`CORS_ORIGINS may not be "*" — list the panel's origin explicitly`)
			}
			origins = append(origins, o)
		}
	}
	return origins, nil
}

func isProduction() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	return env == "production" || env == "prod"
}

func main() {
	godotenv.Load()

	// ── Refuse to start misconfigured ────────────────────────────────────
	required := []string{"DATABASE_URL", "SESSION_SECRET"}
	if isProduction() {
		// Outbound messaging and webhook verification are not optional once
		// this is the real number customers are messaging.
		required = append(required,
			"ACCESS_TOKEN", "PHONE_NUMBER_ID", "VERIFY_TOKEN", "META_APP_SECRET", "PUBLIC_URL")
	}
	if err := requireEnv(required...); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}
	if len(os.Getenv("SESSION_SECRET")) < 32 {
		log.Fatal("Refusing to start: SESSION_SECRET must be at least 32 characters. " +
			"Generate one with: openssl rand -base64 48")
	}

	origins, err := allowedOrigins()
	if err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}

	// ── Database ─────────────────────────────────────────────────────────
	if err := db.InitDB(); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}
	defer db.Pool.Close()
	db.CreateInternalTables()

	if err := EnsureFirstAdmin(); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}

	InitScheduler()

	// ── Router ───────────────────────────────────────────────────────────
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "ngrok-skip-browser-warning"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Pool.Ping(ctx); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, "<h1>%s bot is running</h1>", os.Getenv("COMPANY_NAME"))
	})

	r.HandleFunc("/webhook", WebhookHandler)
	r.Post("/api/login", AuthHandler)
	r.Mount("/api", AdminRoutes())

	// Serve the built panel, falling back to index.html so client-side routes
	// resolve on a hard refresh.
	fs := http.FileServer(http.Dir("./admin-frontend/dist"))
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat("admin-frontend/dist" + r.URL.Path); os.IsNotExist(err) {
			http.ServeFile(w, r, "admin-frontend/dist/index.html")
			return
		}
		fs.ServeHTTP(w, r)
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	// Timeouts, so a slow or abandoned connection cannot hold a handler open
	// indefinitely. ListenAndServe on its own has none of these.
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// ── Serve, and shut down cleanly ─────────────────────────────────────
	// A deploy used to kill the process outright, dropping any webhook still
	// being handled. Meta retries, but a half-written conversation state does
	// not come back.
	go func() {
		log.Printf("🚀 %s starting on port %s (env: %s)", os.Getenv("COMPANY_NAME"), port, os.Getenv("ENV"))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server stopped: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down — finishing in-flight requests…")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Shutdown did not finish cleanly: %v", err)
	}
	log.Println("Stopped.")
}
