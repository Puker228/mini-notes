package backup

import (
	"context"
	"io"
	"time"
)

type Storage interface {
	Save(ctx context.Context) error
	Restore(ctx context.Context, archive io.ReaderAt, size int64) error
}

type Manifest struct {
	CreatedAt  time.Time `json:"created_at"`
	AppVersion string    `json:"app_version"`
	DBFile     string    `json:"db_file"`
	UploadsDir string    `json:"uploads_dir"`
}
