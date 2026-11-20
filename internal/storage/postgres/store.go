package postgres

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"

	_ "github.com/lib/pq"
	"testovoe_ozon/internal/posts"
)

//go:embed schema.sql
var schema string

type Store struct{ db *sql.DB }

func Open(ctx context.Context, dsn string) (*Store, error) {
	if dsn == "" {
		return nil, errors.New("задайте DATABASE_URL")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(20)
	if err := initialize(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func initialize(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Блокировка нужна, чтобы два экземпляра сервиса не создавали таблицы одновременно.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(87642691)"); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, schema); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

const postFields = "id, author_id, body, comments_closed"

func scanPost(row interface{ Scan(...any) error }) (posts.Post, error) {
	var p posts.Post
	err := row.Scan(&p.ID, &p.AuthorID, &p.Text, &p.CommentsClosed)
	return p, err
}

func (s *Store) CreatePost(ctx context.Context, text string, author int64) (posts.Post, error) {
	return scanPost(s.db.QueryRowContext(ctx, "INSERT INTO posts(body,author_id) VALUES ($1,$2) RETURNING "+postFields, text, author))
}

func (s *Store) GetPost(ctx context.Context, id int64) (posts.Post, error) {
	p, err := scanPost(s.db.QueryRowContext(ctx, "SELECT "+postFields+" FROM posts WHERE id=$1", id))
	if errors.Is(err, sql.ErrNoRows) {
		return p, fmt.Errorf("пост с ID %d не найден", id)
	}
	return p, err
}

func (s *Store) ListPosts(ctx context.Context) ([]posts.Post, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+postFields+" FROM posts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]posts.Post, 0)
	for rows.Next() {
		p, err := scanPost(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, rows.Err()
}

func (s *Store) CloseComments(ctx context.Context, id, actor int64) error {
	result, err := s.db.ExecContext(ctx, "UPDATE posts SET comments_closed=true WHERE id=$1 AND author_id=$2", id, actor)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.GetPost(ctx, id); err != nil {
			return err
		}
		return fmt.Errorf("только автор поста %d может закрыть комментарии", id)
	}
	return nil
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func checkParent(ctx context.Context, db rowQuerier, postID int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	var found bool
	err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM comments WHERE post_id=$1 AND id=$2)", postID, *parentID).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("комментарий с ID %d не найден под постом %d", *parentID, postID)
	}
	return nil
}

func (s *Store) CreateComment(ctx context.Context, postID int64, parentID *int64, text string) (posts.Comment, error) {
	if err := posts.ValidateText(text); err != nil {
		return posts.Comment{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return posts.Comment{}, err
	}
	defer tx.Rollback()
	var closed bool
	// Держим пост под блокировкой до записи комментария. Так обсуждение
	// нельзя закрыть между проверкой флага и записью, а ответы получают ID по порядку.
	err = tx.QueryRowContext(ctx, "SELECT comments_closed FROM posts WHERE id=$1 FOR UPDATE", postID).Scan(&closed)
	if errors.Is(err, sql.ErrNoRows) {
		return posts.Comment{}, fmt.Errorf("пост с ID %d не найден", postID)
	}
	if err != nil {
		return posts.Comment{}, err
	}
	if closed {
		return posts.Comment{}, fmt.Errorf("комментарии к посту %d закрыты", postID)
	}
	if err := checkParent(ctx, tx, postID, parentID); err != nil {
		return posts.Comment{}, err
	}
	c := posts.Comment{PostID: postID, Text: text}
	err = tx.QueryRowContext(ctx, "INSERT INTO comments(post_id,parent_id,body) VALUES ($1,$2,$3) RETURNING id,parent_id", postID, parentID, text).Scan(&c.ID, &c.ParentID)
	if err != nil {
		return posts.Comment{}, err
	}
	if err := tx.Commit(); err != nil {
		return posts.Comment{}, err
	}
	return c, nil
}

func (s *Store) ListComments(ctx context.Context, postID int64, parentID *int64, first int, after int64) (posts.CommentPage, error) {
	if err := posts.ValidatePage(first, after); err != nil {
		return posts.CommentPage{}, err
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM posts WHERE id=$1)", postID).Scan(&exists); err != nil {
		return posts.CommentPage{}, err
	}
	if !exists {
		return posts.CommentPage{}, fmt.Errorf("пост с ID %d не найден", postID)
	}
	if err := checkParent(ctx, s.db, postID, parentID); err != nil {
		return posts.CommentPage{}, err
	}

	query := "SELECT id,post_id,parent_id,body FROM comments WHERE post_id=$1 AND id>$2 AND "
	args := []any{postID, after, first + 1}
	if parentID == nil {
		query += "parent_id IS NULL"
	} else {
		query += "parent_id=$4"
		args = append(args, *parentID)
	}
	rows, err := s.db.QueryContext(ctx, query+" ORDER BY id LIMIT $3", args...)
	if err != nil {
		return posts.CommentPage{}, err
	}
	defer rows.Close()
	page := posts.CommentPage{Items: make([]posts.Comment, 0, first)}
	for rows.Next() {
		if len(page.Items) == first {
			page.HasMore = true
			break
		}
		var c posts.Comment
		if err := rows.Scan(&c.ID, &c.PostID, &c.ParentID, &c.Text); err != nil {
			return posts.CommentPage{}, err
		}
		page.Items = append(page.Items, c)
	}
	return page, rows.Err()
}
