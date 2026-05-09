package notes

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) ListNotes(c *gin.Context) {
	notes, err := ListNotes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notes"})
		return
	}

	c.HTML(http.StatusOK, "base.html", gin.H{
		"Notes": notes,
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

	if err := DeleteNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete note"})
		return
	}

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

func readUploadedImage(c *gin.Context) string {
	file, _, err := c.Request.FormFile("image")
	if err != nil {
		return ""
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 10<<20))
	if err != nil || len(data) == 0 {
		return ""
	}
	return base64.StdEncoding.EncodeToString(data)
}

func (h *Handler) CreateNote(c *gin.Context) {
	title := c.PostForm("title")
	content := c.PostForm("content")

	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title required"})
		return
	}

	imageData := readUploadedImage(c)

	note, err := AddNote(title, content, imageData)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create note"})
		return
	}

	c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}

func (h *Handler) UpdateNote(c *gin.Context) {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	title := c.PostForm("title")
	content := c.PostForm("content")

	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title required"})
		return
	}

	imageData := readUploadedImage(c)
	if imageData == "" {
		// keep existing image if no new file uploaded
		existing, err := GetNoteByID(noteID)
		if err == nil {
			imageData = existing.ImageData
		}
	}

	note, err := UpdateNoteByID(noteID, title, content, imageData)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
		return
	}

	c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}
