package notes

type Note struct {
	ID        int64
	Title     string
	Content   string
	ImageData string // base64-encoded image, empty if none
}
