package posts

// Post хранит текст, ID автора и флаг закрытия комментариев
type Post struct {
	AuthorID       int64
	ID             int64
	Text           string
	CommentsClosed bool
}
