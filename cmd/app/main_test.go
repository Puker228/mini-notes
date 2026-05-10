package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func setupCSRFRouter() *echo.Echo {
	router := echo.New()
	router.Use(csrfMiddleware())
	router.GET("/form", func(c *echo.Context) error {
		token, _ := c.Get(middleware.DefaultCSRFConfig.ContextKey).(string)
		return c.String(http.StatusOK, token)
	})
	router.POST("/submit", func(c *echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	return router
}

func TestCSRFConfigSetsTokenCookie(t *testing.T) {
	router := setupCSRFRouter()

	req := httptest.NewRequest(http.MethodGet, "/form", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /form status = %d, want %d", rec.Code, http.StatusOK)
	}
	token := rec.Body.String()
	if token == "" {
		t.Fatal("CSRF token is empty")
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != csrfCookieName || cookie.Value != token {
		t.Fatalf("cookie = %s:%s, want %s:%s", cookie.Name, cookie.Value, csrfCookieName, token)
	}
	if cookie.Path != "/" {
		t.Fatalf("csrf cookie Path = %q, want /", cookie.Path)
	}
	if !cookie.HttpOnly {
		t.Fatal("csrf cookie is not HttpOnly")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("csrf cookie SameSite = %v, want Strict", cookie.SameSite)
	}
}

func TestCSRFConfigRejectsMissingToken(t *testing.T) {
	router := setupCSRFRouter()

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST /submit status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRFConfigAcceptsHeaderToken(t *testing.T) {
	router := setupCSRFRouter()

	token := fetchCSRFToken(t, router)
	postReq := httptest.NewRequest(http.MethodPost, "/submit", nil)
	postReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	postReq.Header.Set(echo.HeaderXCSRFToken, token)
	postRec := httptest.NewRecorder()

	router.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusNoContent {
		t.Fatalf("POST /submit status = %d, want %d", postRec.Code, http.StatusNoContent)
	}
}

func TestCSRFConfigAcceptsFormToken(t *testing.T) {
	router := setupCSRFRouter()

	token := fetchCSRFToken(t, router)
	form := url.Values{csrfFieldName: {token}}
	postReq := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(form.Encode()))
	postReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: token})
	postReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	postRec := httptest.NewRecorder()

	router.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusNoContent {
		t.Fatalf("POST /submit status = %d, want %d", postRec.Code, http.StatusNoContent)
	}
}

func fetchCSRFToken(t *testing.T, router *echo.Echo) string {
	t.Helper()

	getReq := httptest.NewRequest(http.MethodGet, "/form", nil)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /form status = %d, want %d", getRec.Code, http.StatusOK)
	}

	token := getRec.Body.String()
	if token == "" {
		t.Fatal("CSRF token is empty")
	}
	return token
}
