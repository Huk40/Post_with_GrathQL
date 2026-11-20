package memory

import (
	"context"
	"fmt"
	"sort"

	"testovoe_ozon/internal/posts"
)

type commentBranch struct {
	postID   int64
	parentID int64 // Ноль обозначает комментарии верхнего уровня.
}

// CreateComment добавляет комментарий к посту или ответ на другой комментарий.
func (s *Store) CreateComment(_ context.Context, postID int64, parentID *int64, text string) (posts.Comment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, err := s.getPost(postID)
	if err != nil {
		return posts.Comment{}, err
	}
	if post.CommentsClosed {
		return posts.Comment{}, fmt.Errorf("комментарии к посту %d закрыты", postID)
	}
	if err := posts.ValidateText(text); err != nil {
		return posts.Comment{}, err
	}

	branch, err := s.getCommentBranch(postID, parentID)
	if err != nil {
		return posts.Comment{}, err
	}

	s.lastCommentID++
	comment := posts.Comment{
		ID:     s.lastCommentID,
		PostID: postID,
		Text:   text,
	}
	if parentID != nil {
		id := branch.parentID
		comment.ParentID = &id
	}
	s.comments[comment.ID] = comment
	s.commentIDs[branch] = append(s.commentIDs[branch], comment.ID)
	return cloneComment(comment), nil
}

// ListComments читает одну ветку, начиная со следующего ID после afterID.
// Для первой страницы передаём ноль.
func (s *Store) ListComments(_ context.Context, postID int64, parentID *int64, first int, afterID int64) (posts.CommentPage, error) {
	if err := posts.ValidatePage(first, afterID); err != nil {
		return posts.CommentPage{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := s.getPost(postID); err != nil {
		return posts.CommentPage{}, err
	}
	branch, err := s.getCommentBranch(postID, parentID)
	if err != nil {
		return posts.CommentPage{}, err
	}

	ids := s.commentIDs[branch]
	start := sort.Search(len(ids), func(i int) bool {
		return ids[i] > afterID
	})
	end := start + min(first, len(ids)-start)
	items := make([]posts.Comment, 0, end-start)
	for _, id := range ids[start:end] {
		items = append(items, cloneComment(s.comments[id]))
	}
	return posts.CommentPage{Items: items, HasMore: end < len(ids)}, nil
}

// Перед вызовом getCommentBranch нужно взять блокировку хранилища.
func (s *Store) getCommentBranch(postID int64, parentID *int64) (commentBranch, error) {
	branch := commentBranch{postID: postID}
	if parentID != nil {
		id := *parentID
		parent, ok := s.comments[id]
		if !ok || parent.PostID != postID {
			return commentBranch{}, fmt.Errorf("комментарий с ID %d не найден под постом %d", id, postID)
		}
		branch.parentID = id
	}
	return branch, nil
}

// cloneComment копирует ID родителя. Так изменение возвращённого ID не затронет запись в мапе.
func cloneComment(c posts.Comment) posts.Comment {
	if c.ParentID != nil {
		id := *c.ParentID
		c.ParentID = &id
	}
	return c
}
