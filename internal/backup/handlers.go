package backup

import (
	"github.com/labstack/echo/v5"
)

type Handler struct {
	service Storage
}

func NewHandler(service Storage) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Save(c *echo.Context) error {
	ctx := c.Request().Context()
	return h.service.Save(ctx)
}

func (h *Handler) Restore(c *echo.Context) error {
	ctx := c.Request().Context()
	return h.service.Restore(ctx)
}
