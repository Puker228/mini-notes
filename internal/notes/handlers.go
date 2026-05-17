package notes

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const notePreviewRuneLimit = 180

type Handler struct {
	uploadsDir string
}

func NewHandler(uploadsDir string) *Handler {
	return &Handler{uploadsDir: uploadsDir}
}

func withCSRF(c *echo.Context, data map[string]any) map[string]any {
	if data == nil {
		data = map[string]any{}
	}
	token, _ := c.Get(middleware.DefaultCSRFConfig.ContextKey).(string)
	data["CSRFToken"] = token
	return data
}

func renderNoteForDisplay(note Note) (Note, error) {
	if note.IsEncrypted {
		return note, nil
	}
	content, err := renderMD(note.Content)
	if err != nil {
		return Note{}, err
	}
	note.Content = content
	return note, nil
}

func renderNotesForDisplay(result *ListResult) error {
	for i := range result.Notes {
		if result.Notes[i].IsEncrypted {
			continue
		}
		result.Notes[i].Content = notePreview(result.Notes[i].Content)
	}
	return nil
}

func notePreview(content string) string {
	preview := strings.Join(strings.Fields(content), " ")
	if len([]rune(preview)) <= notePreviewRuneLimit {
		return preview
	}

	runes := []rune(preview)
	cut := notePreviewRuneLimit
	for cut > 0 && runes[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = notePreviewRuneLimit
	}
	return strings.TrimSpace(string(runes[:cut])) + "..."
}

func (h *Handler) ListNotes(c *echo.Context) error {
	ctx := c.Request().Context()
	p := ListParams{
		Query:         c.QueryParam("q"),
		Tag:           c.QueryParam("tag"),
		Sort:          c.QueryParam("sort"),
		Order:         c.QueryParam("order"),
		EncryptedOnly: c.QueryParam("encrypted") == "1",
		PageSize:      10,
	}
	if page, err := strconv.Atoi(c.QueryParam("page")); err == nil && page > 0 {
		p.Page = page
	}

	result, err := ListNotes(ctx, p)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to list notes"})
	}
	if err := renderNotesForDisplay(&result); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to render notes"})
	}
	tags, err := ListTags(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to list tags"})
	}

	return c.Render(http.StatusOK, "base.html", withCSRF(c, map[string]any{
		"Result":        result,
		"Query":         p.Query,
		"Tag":           p.Tag,
		"Tags":          tags,
		"Sort":          p.Sort,
		"Order":         p.Order,
		"EncryptedOnly": p.EncryptedOnly,
	}))
}

func (h *Handler) ListArchive(c *echo.Context) error {
	ctx := c.Request().Context()

	notes, err := ListArchivedNotes(ctx)
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
	note, err = renderNoteForDisplay(note)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to render note"})
	}

	return c.Render(http.StatusOK, "detail.html", withCSRF(c, map[string]any{"Note": note}))
}

func (h *Handler) DecryptNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	password := c.FormValue("password")
	if password == "" {
		note, err := GetNoteByID(noteID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.Render(http.StatusBadRequest, "detail.html", withCSRF(c, map[string]any{
			"Note":  note,
			"Error": "Password is required.",
		}))
	}

	note, err := DecryptNoteByID(noteID, password)
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}

		lockedNote, noteErr := GetNoteByID(noteID)
		if noteErr != nil {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.Render(http.StatusUnauthorized, "detail.html", withCSRF(c, map[string]any{
			"Note":  lockedNote,
			"Error": "Wrong password. The note could not be decrypted.",
		}))
	}

	note.IsEncrypted = false
	note, err = renderNoteForDisplay(note)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to render note"})
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

func (h *Handler) TogglePinNote(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	if _, err := TogglePinNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to toggle pin"})
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

	existing, _ := GetNoteByID(noteID)

	if err := PermanentDeleteNoteByID(noteID); err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to permanently delete note"})
	}

	if existing.ImageData != "" {
		_ = os.Remove(filepath.Join(h.uploadsDir, existing.ImageData))
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ShowCreateForm(c *echo.Context) error {
	return c.Render(http.StatusOK, "create.html", withCSRF(c, nil))
}

func (h *Handler) ShowPrivateCreateForm(c *echo.Context) error {
	return c.Render(http.StatusOK, "create_private.html", withCSRF(c, nil))
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

func (h *Handler) UnlockPrivateEditForm(c *echo.Context) error {
	noteID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "invalid note id"})
	}

	password := c.FormValue("password")
	if password == "" {
		note, err := GetNoteByID(noteID)
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.Render(http.StatusBadRequest, "edit.html", withCSRF(c, map[string]any{
			"Note":  note,
			"Error": "Password is required.",
		}))
	}

	note, err := DecryptNoteByID(noteID, password)
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}

		lockedNote, noteErr := GetNoteByID(noteID)
		if noteErr != nil {
			return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
		}
		return c.Render(http.StatusUnauthorized, "edit.html", withCSRF(c, map[string]any{
			"Note":  lockedNote,
			"Error": "Wrong password. The note could not be opened for editing.",
		}))
	}

	return c.Render(http.StatusOK, "edit.html", withCSRF(c, map[string]any{
		"Note":     note,
		"Unlocked": true,
	}))
}

func (h *Handler) resolveUpdatedImage(c *echo.Context, noteID int64) (imageData, oldImageFile string) {
	imageData = h.saveUploadedImage(c)
	if imageData == "" {
		if existing, err := GetNoteByID(noteID); err == nil {
			imageData = existing.ImageData
		}
		return imageData, ""
	}

	if existing, err := GetNoteByID(noteID); err == nil {
		oldImageFile = existing.ImageData
	}
	return imageData, oldImageFile
}

func (h *Handler) saveUploadedImage(c *echo.Context) string {
	file, header, err := c.Request().FormFile("image")
	if err != nil {
		return ""
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)

	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	filename := fmt.Sprintf("%s%s", hex.EncodeToString(b[:]), ext)

	dst, err := os.Create(filepath.Join(h.uploadsDir, filename))
	if err != nil {
		return ""
	}
	defer dst.Close()

	if _, err := io.Copy(dst, io.LimitReader(file, 10<<20)); err != nil {
		_ = os.Remove(dst.Name())
		return ""
	}

	return filename
}

func (h *Handler) CreateNote(c *echo.Context) error {
	title := c.FormValue("title")
	content := c.FormValue("content")
	tags := c.FormValue("tags")

	if title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "title required"})
	}

	note, err := AddNote(title, content, h.saveUploadedImage(c), tags)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to create note"})
	}

	return c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}

func (h *Handler) CreatePrivateNote(c *echo.Context) error {
	title := c.FormValue("title")
	content := c.FormValue("content")
	password := c.FormValue("password")

	if title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "title required"})
	}
	if password == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "password required"})
	}

	note, err := AddPrivateNote(title, content, h.saveUploadedImage(c), password)
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

	existing, err := GetNoteByID(noteID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
	}

	title := c.FormValue("title")
	content := c.FormValue("content")

	if title == "" {
		return c.JSON(http.StatusBadRequest, map[string]any{"error": "title required"})
	}

	imageData, oldImageFile := h.resolveUpdatedImage(c, noteID)

	if existing.IsEncrypted {
		currentPassword := c.FormValue("current_password")
		if currentPassword == "" {
			return c.Render(http.StatusBadRequest, "edit.html", withCSRF(c, map[string]any{
				"Note":     Note{ID: noteID, Title: title, Content: content, ImageData: imageData, IsEncrypted: true},
				"Unlocked": true,
				"Error":    "Current password is required.",
			}))
		}

		note, err := UpdatePrivateNoteByID(noteID, title, content, imageData, currentPassword, c.FormValue("new_password"))
		if err != nil {
			if errors.Is(err, ErrNoteNotFound) {
				return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
			}
			if errors.Is(err, ErrInvalidPassword) {
				return c.Render(http.StatusUnauthorized, "edit.html", withCSRF(c, map[string]any{
					"Note":     Note{ID: noteID, Title: title, Content: content, ImageData: imageData, IsEncrypted: true},
					"Unlocked": true,
					"Error":    "Wrong password. The note was not saved.",
				}))
			}
			return c.JSON(http.StatusInternalServerError, map[string]any{"error": "failed to update note"})
		}

		if oldImageFile != "" {
			_ = os.Remove(filepath.Join(h.uploadsDir, oldImageFile))
		}
		return c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
	}

	note, err := UpdateNoteByID(noteID, title, content, imageData)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]any{"message": "note not found"})
	}

	if oldImageFile != "" {
		_ = os.Remove(filepath.Join(h.uploadsDir, oldImageFile))
	}

	return c.Redirect(http.StatusSeeOther, "/note/"+strconv.FormatInt(note.ID, 10))
}
