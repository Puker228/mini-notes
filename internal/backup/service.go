package backup

import (
	"context"
	"database/sql"
)

type Service struct {
	db         *sql.DB
	uploadsDir string
}

func NewService(db *sql.DB, uploadsDir string) *Service {
	return &Service{
		db:         db,
		uploadsDir: uploadsDir,
	}
}

func (s *Service) Save(ctx context.Context) error    { return nil }
func (s *Service) Restore(ctx context.Context) error { return nil }
