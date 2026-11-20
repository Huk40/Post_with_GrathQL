package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"testovoe_ozon/internal/posts"
)

// Store хранит посты и комментарии в памяти.
type Store struct {
	mu            sync.RWMutex
	posts         map[int64]posts.Post
	lastPostID    int64
	comments      map[int64]posts.Comment
	lastCommentID int64
	commentIDs    map[commentBranch][]int64
}

// NewStore создаёт пустое хранилище постов и комментариев.
func NewStore() *Store {
	return &Store{
		posts:      make(map[int64]posts.Post),
		comments:   make(map[int64]posts.Comment),
		commentIDs: make(map[commentBranch][]int64),
	}
}

// CreatePost сохраняет пост и возвращает его с новым ID.
func (s *Store) CreatePost(_ context.Context, text string, author int64) (posts.Post, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastPostID++

	post := posts.Post{
		AuthorID: author,
		ID:       s.lastPostID,
		Text:     text,
	}

	s.posts[post.ID] = post
	return post, nil
}

func (s *Store) GetPost(_ context.Context, id int64) (posts.Post, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getPost(id)
}

// Перед вызовом getPost нужно взять блокировку хранилища.
func (s *Store) getPost(id int64) (posts.Post, error) {
	post, ok := s.posts[id]
	if !ok {
		return posts.Post{}, fmt.Errorf("пост с ID %d не найден", id)
	}

	return post, nil
}

// CloseComments разрешает закрыть обсуждение только автору поста.
func (s *Store) CloseComments(_ context.Context, postID, actorID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	post, err := s.getPost(postID)
	if err != nil {
		return err
	}
	if actorID != post.AuthorID {
		return fmt.Errorf("только автор поста %d может закрыть комментарии", postID)
	}

	post.CommentsClosed = true
	s.posts[postID] = post
	return nil
}

// ListPosts показывает посты в порядке возрастания ID.
func (s *Store) ListPosts(_ context.Context) ([]posts.Post, error) {
	s.mu.RLock()
	list := make([]posts.Post, 0, len(s.posts))
	for _, post := range s.posts {
		list = append(list, post)
	}
	s.mu.RUnlock()

	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})

	return list, nil
}
