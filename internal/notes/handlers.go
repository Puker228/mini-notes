package notes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) ListNotes(c *gin.Context) {
	c.HTML(http.StatusOK, "base.html", gin.H{
		"Notes": Notes,
	})
}

func (h *Handler) GetNote(c *gin.Context) {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	note, err := GetNoteByID(noteID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
		return
	}

	c.HTML(http.StatusOK, "detail.html", gin.H{
		"Note": note,
	})
}

func (h *Handler) CreateNote(c *gin.Context) {
	newNote := AddNote("Hello", "World")

	c.HTML(http.StatusOK, "base.html", gin.H{
		"newNote": newNote,
	})
}
