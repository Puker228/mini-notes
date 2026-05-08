package notes

type Note struct {
	ID      int64
	Title   string
	Content string
}

var Notes = []Note{
	{ID: 1, Title: "Go basics", Content: "Go is statically typed and compiled."},
	{ID: 2, Title: "Gin routing", Content: "Use gin.Default() for router with logger and recovery."},
	{ID: 3, Title: "REST tips", Content: "Use proper HTTP status codes: 200, 201, 400, 404, 500."},
	{ID: 4, Title: "MongoDB", Content: "mongo-driver v2 uses bson.D for documents and filters."},
	{ID: 5, Title: "Templates", Content: "html/template auto-escapes output to prevent XSS."},
}
