package main

import (
	"embed"
	"html/template"
	"net/http"
	"strconv"

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
	router.GET("/:id", func(c *gin.Context) {
		idParam := c.Param("id")
		noteID, err := strconv.ParseInt(idParam, 10, 64)
		if err != nil {
			c.JSON(400, gin.H{
				"error": "invalid note id",
			})
		}

		note, err := notes.GetNoteByID(noteID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"message": "note not found",
			})
		}

		c.HTML(http.StatusOK, "detail.html", gin.H{
			"Note": note,
		})
	})

	router.GET("/new", func(c *gin.Context) {
		newNote := notes.AddNote("Hello", "World")

		c.HTML(http.StatusOK, "base.html", gin.H{
			"newNote": newNote,
		})
	})

	router.Run(":8000")
}
