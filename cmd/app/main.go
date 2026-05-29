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

type appPaths struct {
	dbPath     string
	uploadsDir string
	backupDir  string
}

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

func configureLogOutput() (io.Writer, func()) {
	logOutput := io.Writer(os.Stdout)
	logPath := os.Getenv("NOTES_LOG_PATH")
	if logPath == "" {
		return logOutput, func() {}
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		log.Fatalf("failed to create log directory: %s", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Fatalf("failed to open log file: %s", err)
	}

	logOutput = io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(logOutput)
	return logOutput, func() {
		if err := logFile.Close(); err != nil {
			log.Println("failed to close log file:", err)
		}
	}
}

func envOrDefault(name, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func ensureDir(path, label string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		log.Fatalf("failed to create %s directory: %s", label, err)
	}
}

func ensureParentDir(path, label string) {
	if dir := filepath.Dir(path); dir != "." {
		ensureDir(dir, label)
	}
}

func loadAppPaths() appPaths {
	paths := appPaths{
		dbPath:     envOrDefault("NOTES_DB_PATH", "notes.db"),
		uploadsDir: envOrDefault("NOTES_UPLOADS_PATH", "uploads"),
		backupDir:  envOrDefault("NOTES_BACKUP_PATH", "backups"),
	}
	ensureParentDir(paths.dbPath, "database")
	ensureDir(paths.uploadsDir, "uploads")
	ensureDir(paths.backupDir, "backups")
	return paths
}

func openDatabase(dbPath string) *sql.DB {
	database, err := sql.Open("sqlite3", "file:"+dbPath)
	if err != nil {
		log.Fatalf("failed to open sqlite database: %s", err)
	}
	return database
}

func closeDatabase(database *sql.DB) {
	if err := database.Close(); err != nil {
		log.Println("failed to close sqlite database:", err)
	}
}

func initNotesDatabase(database *sql.DB) {
	if err := notes.InitDB(database); err != nil {
		log.Fatalf("failed to initialize sqlite database: %s", err)
	}
}

func closeNotesDatabase() {
	if err := notes.CloseDB(); err != nil {
		log.Println("failed to detach sqlite database:", err)
	}
}

func requestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
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
	})
}

func formatDate(v any) string {
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
}

func templateFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatDate": formatDate,
		"urlEncode":  url.QueryEscape,
		"safeHtml":   func(s string) template.HTML { return template.HTML(s) },
	}
}

func configureRenderer(router *echo.Echo) {
	t := template.Must(template.New("").Funcs(templateFuncMap()).ParseFS(templateFS, "templates/*.html"))
	router.Renderer = &echo.TemplateRenderer{Template: t}
}

func configureStaticRoutes(router *echo.Echo, uploadsDir string) {
	staticFS, err := fs.Sub(templateFS, "static")
	if err != nil {
		log.Fatalf("failed to initialize static assets: %s", err)
	}
	router.GET("/static/*", echo.WrapHandler(http.StripPrefix("/static/", http.FileServer(http.FS(staticFS)))))
	router.Static("/uploads", uploadsDir)
}

func registerNoteRoutes(router *echo.Echo, notesHandler *notes.Handler) {
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
}

func registerBackupRoutes(router *echo.Echo, backupHandler *backup.Handler) {
	router.GET("/backup", backupHandler.Save)
	router.POST("/backup/restore", backupHandler.Restore)
}

func buildRouter(logOutput io.Writer, database *sql.DB, paths appPaths) *echo.Echo {
	logger := slog.New(slog.NewJSONHandler(logOutput, nil))
	router := echo.New()
	router.Use(requestLogger(logger))
	router.Use(middleware.Recover())
	router.Use(csrfMiddleware())
	configureRenderer(router)
	configureStaticRoutes(router, paths.uploadsDir)
	registerNoteRoutes(router, notes.NewHandler(paths.uploadsDir))

	backupService := backup.NewService(database, paths.backupDir, paths.uploadsDir)
	registerBackupRoutes(router, backup.NewHandler(backupService))
	return router
}

func newServer(router *echo.Echo) *http.Server {
	return &http.Server{
		Addr:              ":8800",
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func startServer(srv *http.Server) {
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
}

func waitForShutdown(ctx context.Context, stop context.CancelFunc, srv *http.Server) {
	<-ctx.Done()

	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logOutput, closeLog := configureLogOutput()
	defer closeLog()

	paths := loadAppPaths()
	database := openDatabase(paths.dbPath)
	defer closeDatabase(database)
	initNotesDatabase(database)
	defer closeNotesDatabase()

	router := buildRouter(logOutput, database, paths)
	srv := newServer(router)

	startServer(srv)
	waitForShutdown(ctx, stop, srv)
}
