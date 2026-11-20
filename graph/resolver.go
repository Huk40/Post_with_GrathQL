package graph

import (
	"context"
	"crypto/subtle"
	"errors"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"testovoe_ozon/graph/model"
	"testovoe_ozon/internal/posts"
)

type Resolver struct {
	Store  posts.Repository
	Tokens [2]string
}

func (r *Resolver) actor(ctx context.Context) (int64, error) {
	token, ok := strings.CutPrefix(graphql.GetOperationContext(ctx).Headers.Get("Authorization"), "Bearer ")
	if ok && token != "" {
		for i, allowed := range r.Tokens {
			if subtle.ConstantTimeCompare([]byte(token), []byte(allowed)) == 1 {
				return int64(i + 1), nil
			}
		}
	}
	return 0, errors.New("нужен токен автора")
}

func parseID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("ID должен быть положительным целым числом")
	}
	return id, nil
}

func optionalID(value *string) (*int64, error) {
	if value == nil {
		return nil, nil
	}
	id, err := parseID(*value)
	return &id, err
}

func postModel(p posts.Post) *model.Post {
	return &model.Post{ID: strconv.FormatInt(p.ID, 10), Text: p.Text, AuthorID: strconv.FormatInt(p.AuthorID, 10), CommentsClosed: p.CommentsClosed}
}

func commentModel(c posts.Comment) *model.Comment {
	result := &model.Comment{ID: strconv.FormatInt(c.ID, 10), Text: c.Text, PostID: strconv.FormatInt(c.PostID, 10)}
	if c.ParentID != nil {
		parent := strconv.FormatInt(*c.ParentID, 10)
		result.ParentID = &parent
	}
	return result
}
