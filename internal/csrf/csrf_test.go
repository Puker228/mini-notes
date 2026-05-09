package csrf

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupCSRFRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(Middleware())
	router.GET("/form", func(c *gin.Context) {
		c.String(http.StatusOK, Token(c))
	})
	router.POST("/submit", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	return router
}

func TestSetsToken(t *testing.T) {
	router := setupCSRFRouter()

	req := httptest.NewRequest(http.MethodGet, "/form", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /form status = %d, want %d", rec.Code, http.StatusOK)
	}
	token := rec.Body.String()
	if len(token) != 64 {
		t.Fatalf("Token() length = %d, want 64", len(token))
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != CookieName || cookie.Value != token {
		t.Fatalf("cookie = %s:%s, want %s:%s", cookie.Name, cookie.Value, CookieName, token)
	}
	if !cookie.HttpOnly {
		t.Fatal("csrf cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("csrf cookie SameSite = %v, want Strict", cookie.SameSite)
	}
}

func TestRejectsMissingToken(t *testing.T) {
	router := setupCSRFRouter()

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /submit status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestAcceptsHeaderToken(t *testing.T) {
	router := setupCSRFRouter()

	getReq := httptest.NewRequest(http.MethodGet, "/form", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /form status = %d, want %d", getRec.Code, http.StatusOK)
	}

	token := getRec.Body.String()
	postReq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	postReq.AddCookie(&http.Cookie{Name: CookieName, Value: token})
	postReq.Header.Set(HeaderName, token)
	postRec := httptest.NewRecorder()

	router.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusNoContent {
		t.Fatalf("POST /submit status = %d, want %d", postRec.Code, http.StatusNoContent)
	}
}
