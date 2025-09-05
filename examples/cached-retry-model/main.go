package main

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/JoshPattman/jpf"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	// Build model
	fac := &ModelFactory{
		Cache: CreateCache(),
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
}

// Setup a sqlite database and use that as the cache
func CreateCache() jpf.ModelResponseCache {
	db, err := sql.Open("sqlite3", "./cache.db")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	cache, err := jpf.NewSQLModelResponseCache(db)
	if err != nil {
		panic(err)
	}
	return cache
}

// ModelFactory builds models that share the same resources (cache).
// This is the suggested pattern to use with this package.
type ModelFactory struct {
	Cache jpf.ModelResponseCache
}

func (fac *ModelFactory) Build(withCache bool) jpf.Model {
	model := jpf.NewOpenAIModel(os.Getenv("OPENAI_KEY"), "gpt-4o-mini")
	model = jpf.NewRetryModel(model, jpf.WithRetries{X: 5})
	if withCache {
		model = jpf.NewCachedModel(model, fac.Cache)
	}
	return model
}
