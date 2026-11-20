package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"testovoe_ozon/internal/storage/memory"
)

func TestGraphQL(t *testing.T) {
	tokens := [2]string{"test-first", "test-second"}
	handler := newHandler(memory.NewStore(), tokens)
	checkGraphQL(t, "/graphql", tokens, func(req *http.Request) (*http.Response, error) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response.Result(), nil
	})
}

// Для этой проверки нужен запущенный сервис. Его адрес берём из TEST_GRAPHQL_URL.
func TestDeployedGraphQL(t *testing.T) {
	url := os.Getenv("TEST_GRAPHQL_URL")
	if url == "" {
		t.Skip("задайте TEST_GRAPHQL_URL для проверки запущенного сервиса")
	}
	client := http.Client{Timeout: 5 * time.Second}
	checkGraphQL(t, url, [2]string{os.Getenv("TEST_FIRST_TOKEN"), os.Getenv("TEST_SECOND_TOKEN")}, client.Do)
}

func checkGraphQL(t *testing.T, url string, tokens [2]string, send func(*http.Request) (*http.Response, error)) {
	t.Helper()
	call := func(query string, variables map[string]any, token string, wantError bool, dest any) {
		t.Helper()
		payload, err := json.Marshal(map[string]any{"query": query, "variables": variables})
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := send(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var result struct {
			Data   json.RawMessage
			Errors []struct{ Message string }
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if (len(result.Errors) > 0) != wantError {
			t.Fatalf("%s: errors=%v", query, result.Errors)
		}
		if dest != nil && !wantError {
			if err := json.Unmarshal(result.Data, dest); err != nil {
				t.Fatal(err)
			}
		}
	}
	const createPost = `mutation {createPost(text:"test"){id authorID}}`
	var p struct{ CreatePost struct{ ID, AuthorID string } }
	call(createPost, nil, "", true, nil)
	call(createPost, nil, tokens[0], false, &p)
	if p.CreatePost.ID == "" || p.CreatePost.AuthorID != "1" {
		t.Fatalf("post=%+v", p)
	}
	vars := map[string]any{"p": p.CreatePost.ID, "text": "корень"}
	const createComment = `mutation($p:ID!, $parent:ID, $text:String!){createComment(postID:$p,parentID:$parent,text:$text){id}}`
	var c struct{ CreateComment struct{ ID string } }
	call(createComment, vars, "", false, &c)
	root := c.CreateComment.ID
	vars["parent"] = root
	call(createComment, vars, "", false, &c)
	reply := c.CreateComment.ID
	delete(vars, "parent")
	call(createComment, vars, "", false, &c)
	second := c.CreateComment.ID
	vars["text"] = strings.Repeat("я", 2001)
	call(createComment, vars, "", true, nil)
	const read = `query($p:ID!,$parent:ID,$after:ID){post(id:$p){id text commentsClosed} comments(postID:$p,parentID:$parent,after:$after,first:1){items{id} hasMore endCursor}}`
	var page struct {
		Post struct {
			ID             string
			CommentsClosed bool
		}
		Comments struct {
			Items     []struct{ ID string }
			HasMore   bool
			EndCursor string
		}
	}
	checkPage := func(id string, more bool) {
		t.Helper()
		call(read, vars, "", false, &page)
		if page.Post.ID != p.CreatePost.ID || len(page.Comments.Items) != 1 || page.Comments.Items[0].ID != id || page.Comments.EndCursor != id || page.Comments.HasMore != more {
			t.Fatalf("page=%+v", page)
		}
	}
	checkPage(root, true)
	vars["after"] = "0"
	checkPage(root, true)
	vars["after"] = root
	checkPage(second, false)
	delete(vars, "after")
	vars["parent"] = root
	checkPage(reply, false)
	const closePost = `mutation($p:ID!){closeComments(postID:$p)}`
	call(closePost, vars, tokens[1], true, nil)
	vars["text"] = "обсуждение всё ещё открыто"
	call(createComment, vars, "", false, &c)
	call(closePost, vars, tokens[0], false, nil)
	vars["text"] = "после закрытия"
	call(createComment, vars, "", true, nil)
	checkPage(reply, true)
	if !page.Post.CommentsClosed {
		t.Fatal("пост остался открытым")
	}
	var listing struct{ Posts []struct{ ID string } }
	call("{posts{id}}", nil, "", false, &listing)
	for _, item := range listing.Posts {
		if item.ID == p.CreatePost.ID {
			return
		}
	}
	t.Fatal("созданный пост отсутствует в списке")
}
