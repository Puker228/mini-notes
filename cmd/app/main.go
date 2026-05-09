package main

import (
	"context"
	"embed"
	"html/template"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"mini-notes/internal/notes"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	router := gin.Default()

	t := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	router.SetHTMLTemplate(t)

	h := notes.NewHandler()

	router.GET("/note", h.ListNotes)
	router.GET("/note/new", h.ShowCreateForm)
	router.GET("/note/:id", h.GetNote)
	router.POST("/note", h.CreateNote)

	srv := &http.Server{
		Addr:              ":8000",
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}
