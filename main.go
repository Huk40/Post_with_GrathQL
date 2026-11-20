package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"testovoe_ozon/graph"
	"testovoe_ozon/internal/posts"
	"testovoe_ozon/internal/storage/memory"
	"testovoe_ozon/internal/storage/postgres"
)

func main() {
	mode := flag.String("storage", "memory", "memory или postgres")
	addr := flag.String("addr", "127.0.0.1:8080", "адрес HTTP-сервера")
	flag.Parse()
	if err := run(*mode, *addr); err != nil {
		log.Fatal(err)
	}
}

func run(mode, addr string) error {
	tokens := [2]string{os.Getenv("DEMO_AUTHOR_TOKEN"), os.Getenv("SECOND_AUTHOR_TOKEN")}
	if tokens[0] != "" && tokens[0] == tokens[1] {
		return errors.New("токены авторов должны различаться")
	}
	var repo posts.Repository
	switch mode {
	case "memory":
		repo = memory.NewStore()
	case "postgres":
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		store, err := postgres.Open(ctx, os.Getenv("DATABASE_URL"))
		cancel()
		if err != nil {
			return err
		}
		defer store.Close()
		repo = store
	default:
		return errors.New("допустимые значения -storage: memory, postgres")
	}
	server := &http.Server{Addr: addr, Handler: newHandler(repo, tokens), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("GraphQL: http://%s/graphql (%s)", addr, mode)
	return server.ListenAndServe()
}

func newHandler(repo posts.Repository, tokens [2]string) http.Handler {
	resolver := &graph.Resolver{Store: repo, Tokens: tokens}
	server := handler.New(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))
	server.AddTransport(transport.POST{})
	mux := http.NewServeMux()
	mux.Handle("/graphql", server)
	return mux
}
