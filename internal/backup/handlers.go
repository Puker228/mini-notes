package backup

import (
	"net/http"

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
	fileHeader, err := c.FormFile("backup")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "backup file is required")
	}

	file, err := fileHeader.Open()
	if err != nil {
		return err
	}
	defer file.Close()

	return h.service.Restore(c.Request().Context(), file, fileHeader.Size)
}
