package main

import (
	"embed"
	"html/template"

	"mini-notes/internal/notes"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	router := gin.Default()

	t := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	router.SetHTMLTemplate(t)

	h := notes.NewHandler()

	router.GET("/note", h.ListNotes)
	router.GET("/note/new", h.ShowCreateForm)
	router.GET("/note/:id", h.GetNote)
	router.POST("/note", h.CreateNote)

	router.Run(":8000")
}
