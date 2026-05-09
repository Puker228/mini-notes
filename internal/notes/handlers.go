package notes

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"

	"mini-notes/internal/csrf"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func withCSRF(c *gin.Context, data gin.H) gin.H {
	if data == nil {
		data = gin.H{}
	}
	data["CSRFToken"] = csrf.Token(c)
	return data
}

func (h *Handler) ListNotes(c *gin.Context) {
	p := ListParams{
		Query:    c.Query("q"),
		Sort:     c.Query("sort"),
		Order:    c.Query("order"),
		PageSize: 10,
	}
	if page, err := strconv.Atoi(c.Query("page")); err == nil && page > 0 {
		p.Page = page
	}

	result, err := ListNotes(p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list notes"})
		return
	}

	c.HTML(http.StatusOK, "base.html", withCSRF(c, gin.H{
		"Result": result,
		"Query":  p.Query,
		"Sort":   p.Sort,
		"Order":  p.Order,
	}))
}

func (h *Handler) ListArchive(c *gin.Context) {
	notes, err := ListArchivedNotes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list archive"})
		return
	}
	c.HTML(http.StatusOK, "archive.html", withCSRF(c, gin.H{"Notes": notes}))
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

	c.HTML(http.StatusOK, "detail.html", withCSRF(c, gin.H{"Note": note}))
}

func (h *Handler) DeleteNote(c *gin.Context) {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	if err := SoftDeleteNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete note"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) RestoreNote(c *gin.Context) {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	if err := RestoreNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to restore note"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) PermanentDeleteNote(c *gin.Context) {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid note id"})
		return
	}

	if err := PermanentDeleteNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"message": "note not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to permanently delete note"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) ShowCreateForm(c *gin.Context) {
	c.HTML(http.StatusOK, "create.html", withCSRF(c, nil))
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

	c.HTML(http.StatusOK, "edit.html", withCSRF(c, gin.H{"Note": note}))
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

	note, err := AddNote(title, content, readUploadedImage(c))
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
		if existing, err := GetNoteByID(noteID); err == nil {
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
