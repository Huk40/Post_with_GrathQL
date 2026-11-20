package posts

import (
	"fmt"
	"unicode/utf8"
)

const maxCommentLength = 2000

// Comment хранит текст и ID поста. У ответа есть ID родительского комментария
type Comment struct {
	ID       int64
	PostID   int64
	ParentID *int64
	Text     string
}

// CommentPage содержит одну страницу ветки. HasMore - есть ли ещё комментарии.
type CommentPage struct {
	Items   []Comment
	HasMore bool
}

func ValidateText(text string) error {
	if utf8.RuneCountInString(text) > maxCommentLength {
		return fmt.Errorf("комментарий длиннее %d символов", maxCommentLength)
	}
	return nil
}

func ValidatePage(first int, afterID int64) error {
	if first < 1 || first > 100 || afterID < 0 {
		return fmt.Errorf("размер страницы должен быть от 1 до 100, курсор — неотрицательным")
	}
	return nil
}
