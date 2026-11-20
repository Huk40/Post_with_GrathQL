package posts

import "context"

type Repository interface {
	CreatePost(context.Context, string, int64) (Post, error)
	GetPost(context.Context, int64) (Post, error)
	ListPosts(context.Context) ([]Post, error)
	CloseComments(context.Context, int64, int64) error
	CreateComment(context.Context, int64, *int64, string) (Comment, error)
	ListComments(context.Context, int64, *int64, int, int64) (CommentPage, error)
}
