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

func (h *Handler) DeleteNote(c *gin.Context) {
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

	DeleteNoteByID(note.ID)
	c.Status(http.StatusNoContent)
}

func (h *Handler) ShowCreateForm(c *gin.Context) {
	c.HTML(http.StatusOK, "create.html", nil)
}

func (h *Handler) ShowEditForm(c *gin.Context) {
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

	c.HTML(http.StatusOK, "edit.html", gin.H{
		"Note": note,
	})
}

func (h *Handler) CreateNote(c *gin.Context) {
	title := c.PostForm("title")
	content := c.PostForm("content")

	if title == "" || content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and content required"})
		return
	}

	AddNote(title, content)
	c.Redirect(http.StatusSeeOther, "/note")
}

func (h *Handler) UpdateNote(c *gin.Context) {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	title := c.PostForm("title")
	content := c.PostForm("content")

	if title == "" || content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title and content required"})
		return
	}

	note, err := UpdateNoteByID(noteID, title, content)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
		return
	}

	c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}
