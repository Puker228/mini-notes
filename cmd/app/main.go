package main

import (
	"context"
	"database/sql"
	"embed"
	"html/template"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Puker228/mini-notes/internal/backup"
	"github.com/Puker228/mini-notes/internal/notes"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	_ "github.com/ncruces/go-sqlite3/driver"
)

//go:embed templates/* static/*
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

	logOutput := io.Writer(os.Stdout)
	logPath := os.Getenv("NOTES_LOG_PATH")
	if logPath != "" {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			log.Fatalf("failed to create log directory: %s", err)
		}
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("failed to open log file: %s", err)
		}
		defer func() {
			if err := logFile.Close(); err != nil {
				log.Println("failed to close log file:", err)
			}
		}()
		logOutput = io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(logOutput)
	}

	dbPath := os.Getenv("NOTES_DB_PATH")
	if dbPath == "" {
		dbPath = "notes.db"
	}
	if dbDir := filepath.Dir(dbPath); dbDir != "." {
		if err := os.MkdirAll(dbDir, 0o755); err != nil {
			log.Fatalf("failed to create database directory: %s", err)
		}
	}

	uploadsDir := os.Getenv("NOTES_UPLOADS_PATH")
	if uploadsDir == "" {
		uploadsDir = "uploads"
	}
	if err := os.MkdirAll(uploadsDir, 0o755); err != nil {
		log.Fatalf("failed to create uploads directory: %s", err)
	}

	backupDir := os.Getenv("NOTES_BACKUP_PATH")
	if backupDir == "" {
		backupDir = "backups"
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		log.Fatalf("failed to create backups directory: %s", err)
	}

	database, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		log.Fatalf("failed to open sqlite database: %s", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Println("failed to close sqlite database:", err)
		}
	}()

	if err := notes.InitDB(database); err != nil {
		log.Fatalf("failed to initialize sqlite database: %s", err)
	}
	defer func() {
		if err := notes.CloseDB(); err != nil {
			log.Println("failed to detach sqlite database:", err)
		}
	}()

	logger := slog.New(slog.NewJSONHandler(logOutput, nil))

	router := echo.New()
	router.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:   true,
		LogURI:      true,
		HandleError: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error == nil {
				logger.LogAttrs(
					context.Background(), slog.LevelInfo, "REQUEST",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.Duration("latency", v.Latency),
				)
			} else {
				logger.LogAttrs(
					context.Background(), slog.LevelError, "REQUEST_ERROR",
					slog.String("uri", v.URI),
					slog.Int("status", v.Status),
					slog.String("err", v.Error.Error()),
				)
			}
			return nil
		},
	}))
	// router.Use(middleware.RequestLogger())
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
		"safeHtml":  func(s string) template.HTML { return template.HTML(s) },
	}

	t := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	router.Renderer = &echo.TemplateRenderer{Template: t}

	staticFS, err := fs.Sub(templateFS, "static")
	if err != nil {
		log.Fatalf("failed to initialize static assets: %s", err)
	}
	router.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))
	router.Static("/uploads", uploadsDir)

	notesHandler := notes.NewHandler(uploadsDir)
	router.GET("/note", notesHandler.ListNotes)
	router.GET("/note/new", notesHandler.ShowCreateForm)
	router.GET("/note/private/new", notesHandler.ShowPrivateCreateForm)
	router.GET("/note/:id/edit", notesHandler.ShowEditForm)
	router.GET("/note/:id", notesHandler.GetNote)
	router.POST("/note", notesHandler.CreateNote)
	router.POST("/note/private", notesHandler.CreatePrivateNote)
	router.POST("/note/:id/decrypt", notesHandler.DecryptNote)
	router.POST("/note/:id/edit/unlock", notesHandler.UnlockPrivateEditForm)
	router.POST("/note/:id/edit", notesHandler.UpdateNote)
	router.POST("/note/:id/pin", notesHandler.TogglePinNote)
	router.DELETE("/note/:id", notesHandler.DeleteNote)
	router.POST("/note/:id/restore", notesHandler.RestoreNote)
	router.DELETE("/note/:id/permanent", notesHandler.PermanentDeleteNote)
	router.GET("/archive", notesHandler.ListArchive)

	backupService := backup.NewService(database, backupDir, uploadsDir)
	backupHandler := backup.NewHandler(backupService)
	router.GET("/backup", backupHandler.Save)
	router.POST("/backup/restore", backupHandler.Restore)

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
