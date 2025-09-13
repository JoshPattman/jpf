package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/JoshPattman/jpf"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Build model
	fac := &ModelFactory{
		Cache:  CreateCache(),
		Logger: slog.Default(),
	}
	model := fac.Build(true)
	// Time first request (should take about ~1 second)
	now := time.Now()
	model.Respond([]jpf.Message{{Role: jpf.SystemRole, Content: "abc"}})
	t1 := time.Since(now)
	// Time second request (should be instant)
	now = time.Now()
	model.Respond([]jpf.Message{{Role: jpf.SystemRole, Content: "abc"}})
	t2 := time.Since(now)
	fmt.Println(t1, t2)

	emb := fac.BuildEmbedder(true)
	// Do the same for embedder
	now = time.Now()
	_, err := emb.Embed("abc")
	if err != nil {
		panic(err)
	}
	t1 = time.Since(now)
	// Time second request (should be instant)
	now = time.Now()
	emb.Embed("abc")
	t2 = time.Since(now)
	fmt.Println(t1, t2)
}

// Setup a sqlite database and use that as the cache
func CreateCache() jpf.Cache {
	db, err := sql.Open("sqlite3", "./cache.db")
	if err != nil {
		panic(err)
	}
	cache, err := jpf.NewSQLCache(db)
	if err != nil {
		panic(err)
	}
	return cache
}

// ModelFactory builds models that share the same resources (cache).
// This is the suggested pattern to use with this package.
type ModelFactory struct {
	Cache  jpf.Cache
	Logger *slog.Logger
}

func (fac *ModelFactory) Build(withCache bool) jpf.Model {
	model := jpf.NewOpenAIModel(os.Getenv("OPENAI_KEY"), "gpt-4o-mini")
	model = jpf.NewRetryModel(model, 5)
	if withCache {
		model = jpf.NewCachedModel(model, fac.Cache)
	}
	if fac.Logger != nil {
		model = jpf.NewLoggingModel(model, jpf.NewSlogModelLogger(fac.Logger.Info, true))
	}
	return model
}

func (fac *ModelFactory) BuildEmbedder(withCache bool) jpf.Embedder {
	emb := jpf.NewOpenAIEmbedder(os.Getenv("OPENAI_KEY"), "text-embedding-3-small")
	if withCache {
		emb = jpf.NewCachedEmbedder(emb, fac.Cache)
	}
	return emb
}
