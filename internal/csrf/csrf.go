package csrf

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	CookieName = "csrf_token"
	HeaderName = "X-CSRF-Token"
	FieldName  = "_csrf"
	ContextKey = "csrf_token"
)

func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("csrf: failed to generate token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(CookieName)
		if err != nil || token == "" {
			token = generateToken()
		}

		http.SetCookie(c.Writer, &http.Cookie{
			Name:     CookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		c.Set(ContextKey, token)

		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			submitted := c.GetHeader(HeaderName)
			if submitted == "" {
				submitted = c.PostForm(FieldName)
			}
			if submitted != token {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		c.Next()
	}
}

// Token returns the CSRF token stored in the Gin context.
func Token(c *gin.Context) string {
	token, _ := c.Get(ContextKey)
	s, _ := token.(string)
	return s
}
