package memory

import (
	"context"
	"strings"
	"sync"
	"testing"

	"testovoe_ozon/internal/posts"
)

func TestCommentBranches(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	post, _ := s.CreatePost(ctx, "первый", 1)
	other, _ := s.CreatePost(ctx, "второй", 2)
	add := func(parent *int64, text string) posts.Comment {
		t.Helper()
		c, err := s.CreateComment(ctx, post.ID, parent, text)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a := add(nil, strings.Repeat("я", 2000))
	reply := add(&a.ID, "ответ")
	b := add(nil, "ещё один")
	nested := add(&reply.ID, "ответ на ответ")
	if _, err := s.CreateComment(ctx, post.ID, nil, strings.Repeat("я", 2001)); err == nil {
		t.Fatal("разрешён длинный текст")
	}
	if _, err := s.CreateComment(ctx, other.ID, &a.ID, "чужая ветка"); err == nil {
		t.Fatal("родитель из другого поста")
	}
	check := func(parent *int64, after int64, wantID int64, more bool) {
		t.Helper()
		page, err := s.ListComments(ctx, post.ID, parent, 1, after)
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != wantID || page.HasMore != more {
			t.Fatalf("page=%+v, err=%v", page, err)
		}
	}
	check(nil, 0, a.ID, true)
	check(nil, a.ID, b.ID, false)
	check(&a.ID, 0, reply.ID, false)
	check(&reply.ID, 0, nested.ID, false)
	page, err := s.ListComments(ctx, post.ID, nil, 1, b.ID)
	if err != nil || len(page.Items) != 0 || page.HasMore {
		t.Fatalf("last page: %+v %v", page, err)
	}
	if err := s.CloseComments(ctx, post.ID, 2); err == nil {
		t.Fatal("чужой пользователь закрыл пост")
	}
	if err := s.CloseComments(ctx, post.ID, 1); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []*int64{nil, &a.ID} {
		if _, err := s.CreateComment(ctx, post.ID, parent, "поздно"); err == nil {
			t.Fatal("новый комментарий после закрытия")
		}
	}
	check(&a.ID, 0, reply.ID, false)
}

func TestConcurrentCreation(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	p, _ := s.CreatePost(ctx, "пост", 1)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.CreateComment(ctx, p.ID, nil, "комментарий"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	page, err := s.ListComments(ctx, p.ID, nil, 100, 0)
	if err != nil || len(page.Items) != 64 {
		t.Fatalf("size=%d err=%v", len(page.Items), err)
	}
	for i, c := range page.Items {
		if c.ID != int64(i+1) {
			t.Fatalf("ID=%d, position=%d", c.ID, i)
		}
	}
}

func TestDeepBranchAndValidation(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	p, err := s.CreatePost(ctx, "обсуждение", 1)
	if err != nil {
		t.Fatal(err)
	}
	var parent *int64
	var c posts.Comment
	for i := 0; i < 4000; i++ {
		c, err = s.CreateComment(ctx, p.ID, parent, "ответ")
		if err != nil {
			t.Fatal(err)
		}
		id := c.ID
		parent = &id
	}
	page, err := s.ListComments(ctx, p.ID, c.ParentID, 10, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != c.ID {
		t.Fatalf("deep branch: page=%v, err=%v", page, err)
	}
	missing := c.ID + 1
	for _, tc := range []struct {
		post   int64
		parent *int64
		first  int
		after  int64
	}{
		{p.ID, nil, 0, 0}, {p.ID, nil, 101, 0}, {p.ID, nil, 1, -1},
		{p.ID, &missing, 1, 0}, {p.ID + 1, nil, 1, 0},
	} {
		if _, err := s.ListComments(ctx, tc.post, tc.parent, tc.first, tc.after); err == nil {
			t.Errorf("expected validation error: %+v", tc)
		}
	}
}
