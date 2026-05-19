package backup

import "context"

type Service struct {
	dbPath     string
	uploadsDir string
}

func NewService(dbPath, uploadsDir string) *Service {
	return &Service{
		dbPath:     dbPath,
		uploadsDir: uploadsDir,
	}
}

func (s *Service) Save(ctx context.Context) error    { return nil }
func (s *Service) Restore(ctx context.Context) error { return nil }
