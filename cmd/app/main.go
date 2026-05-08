package main

import (
	"embed"
	"html/template"
	"net/http"

	"mini-notes/internal/notes"

	"github.com/gin-gonic/gin"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	router := gin.Default()

	t := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	router.SetHTMLTemplate(t)

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "base.html", gin.H{
			"Notes": notes.Notes,
		})
	})

	router.Run(":8000")
}
