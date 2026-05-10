package notes

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func withCSRF(c *echo.Context, data map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	token, _ := c.Get(middleware.DefaultCSRFConfig.ContextKey).(string)
	data["CSRFToken"] = token
	return data
}

func (h *Handler) ListNotes(c *echo.Context) error {
	p := ListParams{
		Query:    c.QueryParam("q"),
		Sort:     c.QueryParam("sort"),
		Order:    c.QueryParam("order"),
		PageSize: 10,
	}
	if page, err := strconv.Atoi(c.QueryParam("page")); err == nil && page > 0 {
		p.Page = page
	}

	result, err := ListNotes(p)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to list notes"})
	}

	return c.Render(http.StatusOK, "base.html", withCSRF(c, map[string]any{
		"Result": result,
		"Query":  p.Query,
		"Sort":   p.Sort,
		"Order":  p.Order,
	}))
}

func (h *Handler) ListArchive(c *echo.Context) error {
	notes, err := ListArchivedNotes()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to list archive"})
	}
	return c.Render(http.StatusOK, "archive.html", withCSRF(c, map[string]any{"Notes": notes}))
}

func (h *Handler) GetNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	note, err := GetNoteByID(noteID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
	}

	return c.Render(http.StatusOK, "detail.html", withCSRF(c, map[string]any{"Note": note}))
}

func (h *Handler) DeleteNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	if err := SoftDeleteNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to delete note"})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) RestoreNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	if err := RestoreNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to restore note"})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PermanentDeleteNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	if err := PermanentDeleteNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to permanently delete note"})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ShowCreateForm(c *echo.Context) error {
	return c.Render(http.StatusOK, "create.html", withCSRF(c, nil))
}

func (h *Handler) ShowEditForm(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	note, err := GetNoteByID(noteID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
	}

	return c.Render(http.StatusOK, "edit.html", withCSRF(c, map[string]any{"Note": note}))
}

func readUploadedImage(c *echo.Context) string {
	file, _, err := c.Request().FormFile("image")
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

func (h *Handler) CreateNote(c *echo.Context) error {
	title := c.FormValue("title")
	content := c.FormValue("content")

	if title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "title required"})
	}

	note, err := AddNote(title, content, readUploadedImage(c))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to create note"})
	}

	return c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}

func (h *Handler) UpdateNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	title := c.FormValue("title")
	content := c.FormValue("content")

	if title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "title required"})
	}

	imageData := readUploadedImage(c)
	if imageData == "" {
		if existing, err := GetNoteByID(noteID); err == nil {
			imageData = existing.ImageData
		}
	}

	note, err := UpdateNoteByID(noteID, title, content, imageData)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
	}

	return c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}
