package notes

import "time"

type Note struct {
	ID          int64
	Title       string
	Content     string
	ImageData   string
	IsPinned    bool
	IsEncrypted bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type ListParams struct {
	Query         string
	Sort          string // "created_at", "updated_at", "title"
	Order         string // "asc", "desc"
	EncryptedOnly bool
	Page          int
	PageSize      int
}

type ListResult struct {
	Notes      []Note
	Total      int
	Page       int
	PageSize   int
	TotalPages int
	HasPrev    bool
	HasNext    bool
	PrevPage   int
	NextPage   int
}
