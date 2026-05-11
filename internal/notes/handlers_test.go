package notes

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/labstack/echo/v5"
)

type testRenderer struct {
	templates *template.Template
}

func (r *testRenderer) Render(_ *echo.Context, w io.Writer, name string, data any) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func setupHandlerRouter(t *testing.T) *echo.Echo {
	t.Helper()

	setupTestDB(t)

	router := echo.New()
	router.Renderer = &testRenderer{
		templates: template.Must(template.New("").Funcs(template.FuncMap{
			"formatDate": func(v any) string {
				switch t := v.(type) {
				case time.Time:
					if t.IsZero() {
						return "—"
					}
					return t.Format("02.01.2006 15:04")
				case *time.Time:
					if t == nil || t.IsZero() {
						return "—"
					}
					return t.Format("02.01.2006 15:04")
				}
				return "—"
			},
			"urlEncode": url.QueryEscape,
		}).ParseGlob("../../cmd/app/templates/*.html")),
	}
	h := NewHandler(t.TempDir())
	router.POST("/note", h.CreateNote)
	router.POST("/note/private", h.CreatePrivateNote)
	router.POST("/note/:id/decrypt", h.DecryptNote)
	router.POST("/note/:id/edit", h.UpdateNote)
	router.DELETE("/note/:id", h.DeleteNote)
	router.POST("/note/:id/restore", h.RestoreNote)
	router.DELETE("/note/:id/permanent", h.PermanentDeleteNote)
	return router
}

func TestCreateNoteWithImage(t *testing.T) {
	router := setupHandlerRouter(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("title", "handler title"); err != nil {
		t.Fatalf("WriteField(title) error = %v", err)
	}
	if err := writer.WriteField("content", "handler content"); err != nil {
		t.Fatalf("WriteField(content) error = %v", err)
	}
	file, err := writer.CreateFormFile("image", "note.png")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := file.Write([]byte("image-bytes")); err != nil {
		t.Fatalf("file.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/note", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("CreateNote status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/note/1" {
		t.Fatalf("CreateNote Location = %q, want /note/1", got)
	}

	result, err := ListNotes(ListParams{})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if result.Total != 1 || len(result.Notes) != 1 {
		t.Fatalf("ListNotes() = %+v", result)
	}
	note := result.Notes[0]
	if note.Title != "handler title" || note.Content != "handler content" {
		t.Fatalf("created note = %+v", note)
	}
	if note.ImageData == "" {
		t.Fatalf("created note image data is empty, expected a filename")
	}
}

func TestCreatePrivateNote(t *testing.T) {
	router := setupHandlerRouter(t)

	form := url.Values{
		"title":    {"private title"},
		"content":  {"private content"},
		"password": {"secret"},
	}
	req := httptest.NewRequest(http.MethodPost, "/note/private", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("CreatePrivateNote status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if got := rec.Header().Get("Location"); got != "/note/1" {
		t.Fatalf("CreatePrivateNote Location = %q, want /note/1", got)
	}

	var storedContent string
	var isEncrypted bool
	if err := db.QueryRow(`SELECT content, is_encypted FROM notes WHERE id = 1;`).Scan(&storedContent, &isEncrypted); err != nil {
		t.Fatalf("QueryRow() error = %v", err)
	}
	if storedContent == "private content" {
		t.Fatalf("stored content is plaintext, want encrypted content")
	}
	if !isEncrypted {
		t.Fatalf("is_encypted = false, want true")
	}

	result, err := ListNotes(ListParams{})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if len(result.Notes) != 1 || !result.Notes[0].IsEncrypted {
		t.Fatalf("ListNotes() note = %+v, want encrypted note", result.Notes)
	}
}

func TestCreatePrivateNoteNeedsPassword(t *testing.T) {
	router := setupHandlerRouter(t)

	form := url.Values{
		"title":   {"private title"},
		"content": {"private content"},
	}
	req := httptest.NewRequest(http.MethodPost, "/note/private", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("CreatePrivateNote status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDecryptPrivateNote(t *testing.T) {
	router := setupHandlerRouter(t)

	created, err := AddPrivateNote("private title", "private content", "", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	form := url.Values{"password": {"secret"}}
	req := httptest.NewRequest(http.MethodPost, "/note/"+strconv.FormatInt(created.ID, 10)+"/decrypt", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("DecryptNote status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("private content")) {
		t.Fatalf("DecryptNote body does not contain decrypted content: %s", rec.Body.String())
	}
}

func TestDecryptPrivateNoteWrongPassword(t *testing.T) {
	router := setupHandlerRouter(t)

	created, err := AddPrivateNote("private title", "private content", "", "secret")
	if err != nil {
		t.Fatalf("AddPrivateNote() error = %v", err)
	}

	form := url.Values{"password": {"wrong"}}
	req := httptest.NewRequest(http.MethodPost, "/note/"+strconv.FormatInt(created.ID, 10)+"/decrypt", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("DecryptNote status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("Wrong password")) {
		t.Fatalf("DecryptNote body does not contain password error: %s", rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("private content")) {
		t.Fatalf("DecryptNote body contains decrypted content after wrong password: %s", rec.Body.String())
	}
}

func TestCreateNoteNeedsTitle(t *testing.T) {
	router := setupHandlerRouter(t)

	form := url.Values{"content": {"content without title"}}
	req := httptest.NewRequest(http.MethodPost, "/note", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("CreateNote status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	result, err := ListNotes(ListParams{})
	if err != nil {
		t.Fatalf("ListNotes() error = %v", err)
	}
	if result.Total != 0 {
		t.Fatalf("ListNotes() total = %d, want 0", result.Total)
	}
}

func TestUpdateNoteKeepsImage(t *testing.T) {
	router := setupHandlerRouter(t)

	created, err := AddNote("old title", "old content", "old-image")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	form := url.Values{
		"title":   {"new title"},
		"content": {"new content"},
	}
	req := httptest.NewRequest(http.MethodPost, "/note/"+strconv.FormatInt(created.ID, 10)+"/edit", bytes.NewBufferString(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("UpdateNote status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	got, err := GetNoteByID(created.ID)
	if err != nil {
		t.Fatalf("GetNoteByID() error = %v", err)
	}
	if got.Title != "new title" || got.Content != "new content" || got.ImageData != "old-image" {
		t.Fatalf("updated note = %+v", got)
	}
}

func TestDeleteRestorePermanent(t *testing.T) {
	router := setupHandlerRouter(t)

	created, err := AddNote("title", "content", "")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}
	notePath := "/note/" + strconv.FormatInt(created.ID, 10)

	req := httptest.NewRequest(http.MethodDelete, notePath, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DeleteNote status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if _, err := GetNoteByID(created.ID); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() after delete error = %v, want %v", err, ErrNoteNotFound)
	}

	req = httptest.NewRequest(http.MethodPost, notePath+"/restore", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("RestoreNote status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if _, err := GetNoteByID(created.ID); err != nil {
		t.Fatalf("GetNoteByID() after restore error = %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete, notePath+"/permanent", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("PermanentDeleteNote status = %d, want %d", rec.Code, http.StatusNoContent)
	}
	if _, err := GetNoteByID(created.ID); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("GetNoteByID() after permanent delete error = %v, want %v", err, ErrNoteNotFound)
	}
}

func TestMutationHandlerErrors(t *testing.T) {
	router := setupHandlerRouter(t)

	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{name: "delete bad id", method: http.MethodDelete, path: "/note/not-a-number", want: http.StatusBadRequest},
		{name: "delete missing", method: http.MethodDelete, path: "/note/404", want: http.StatusNotFound},
		{name: "restore bad id", method: http.MethodPost, path: "/note/not-a-number/restore", want: http.StatusBadRequest},
		{name: "restore missing", method: http.MethodPost, path: "/note/404/restore", want: http.StatusNotFound},
		{name: "permanent bad id", method: http.MethodDelete, path: "/note/not-a-number/permanent", want: http.StatusBadRequest},
		{name: "permanent missing", method: http.MethodDelete, path: "/note/404/permanent", want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Fatalf("%s %s status = %d, want %d", tt.method, tt.path, rec.Code, tt.want)
			}
		})
	}
}
