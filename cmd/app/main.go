package main

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mini-notes/internal/notes"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

//go:embed templates/*
var templateFS embed.FS

const (
	csrfCookieName = "csrf_token"
	csrfFieldName  = "_csrf"
)

func csrfMiddleware() echo.MiddlewareFunc {
	return middleware.CSRFWithConfig(middleware.CSRFConfig{
		TokenLookup:    "header:" + echo.HeaderXCSRFToken + ",form:" + csrfFieldName,
		CookieName:     csrfCookieName,
		CookiePath:     "/",
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		ErrorHandler: func(_ *echo.Context, _ error) error {
			return middleware.ErrCSRFInvalid
		},
	})
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dbPath := os.Getenv("NOTES_DB_PATH")
	if dbPath == "" {
		dbPath = "notes.db"
	}

	uploadsDir := os.Getenv("NOTES_UPLOADS_PATH")
	if uploadsDir == "" {
		uploadsDir = "uploads"
	}
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("failed to create uploads directory: %s", err)
	}

	if err := notes.InitDB(dbPath); err != nil {
		log.Fatalf("failed to initialize sqlite database: %s", err)
	}
	defer func() {
		if err := notes.CloseDB(); err != nil {
			log.Println("failed to close sqlite database:", err)
		}
	}()

	router := echo.New()
	router.Use(middleware.RequestLogger())
	router.Use(middleware.Recover())
	router.Use(csrfMiddleware())

	funcMap := template.FuncMap{
		"formatDate": func(v any) string {
			switch t := v.(type) {
			case time.Time:
				if t.IsZero() {
					return "—"
				}
				return t.Format("02.01.2006 15:04")
			case *time.Time:
				if t == nil || t.IsZero() {
					return "—"
				}
				return t.Format("02.01.2006 15:04")
			}
			return "—"
		},
		"urlEncode": url.QueryEscape,
	}

	t := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	router.Renderer = &echo.TemplateRenderer{Template: t}

	router.Static("/uploads", uploadsDir)

	h := notes.NewHandler(uploadsDir)

	router.GET("/note", h.ListNotes)
	router.GET("/note/new", h.ShowCreateForm)
	router.GET("/note/:id/edit", h.ShowEditForm)
	router.GET("/note/:id", h.GetNote)
	router.POST("/note", h.CreateNote)
	router.POST("/note/:id/edit", h.UpdateNote)
	router.DELETE("/note/:id", h.DeleteNote)
	router.POST("/note/:id/restore", h.RestoreNote)
	router.DELETE("/note/:id/permanent", h.PermanentDeleteNote)
	router.GET("/archive", h.ListArchive)

	srv := &http.Server{
		Addr:              ":8800",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-ctx.Done()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
