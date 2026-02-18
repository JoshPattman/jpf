package caches

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"errors"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

func NewSQL(ctx context.Context, db *sql.DB) (jpf.ModelResponseCache, error) {
	c := &sqlCache{
		db: db,
	}
	err := c.setupDB(ctx)
	if err != nil {
		return nil, err
	}
	return c, nil
}

type sqlCache struct {
	db *sql.DB
}

func (cache *sqlCache) GetCachedResponse(ctx context.Context, salt string, msgs []jpf.Message) (bool, []jpf.Message, jpf.Message, error) {
	h := HashMessages(salt, msgs)
	row := cache.db.QueryRowContext(ctx, `SELECT resp FROM model_cache WHERE hash=?;`, h)
	blob := []byte{}
	err := row.Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, jpf.Message{}, nil
	} else if err != nil {
		return false, nil, jpf.Message{}, utils.Wrap(err, "failed to query database")
	}
	var outputs []jpf.Message
	err = gob.NewDecoder(bytes.NewBuffer(blob)).Decode(&outputs)
	if err != nil {
		return false, nil, jpf.Message{}, utils.Wrap(err, "failed to decode cached data")
	}
	if len(outputs) == 0 {
		return false, nil, jpf.Message{}, errors.New("cached messages have 0 length")
	}
	return true, outputs[:len(outputs)-1], outputs[len(outputs)-1], nil
}

func (cache *sqlCache) SetCachedResponse(ctx context.Context, salt string, inputs []jpf.Message, aux []jpf.Message, out jpf.Message) error {
	h := HashMessages(salt, inputs)
	blob := bytes.NewBuffer(nil)
	err := gob.NewEncoder(blob).Encode(append(aux, out))
	if err != nil {
		return utils.Wrap(err, "failed to encode messages to binary data")
	}
	_, err = cache.db.ExecContext(ctx, `INSERT INTO model_cache (hash, resp) VALUES (?, ?) ON CONFLICT(hash) DO UPDATE SET resp = excluded.resp;`, h, blob.Bytes())
	if err != nil {
		return utils.Wrap(err, "failed to execute database insert")
	}
	return nil
}

func (cache *sqlCache) setupDB(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS model_cache (
		hash TEXT PRIMARY KEY,
		resp BLOB NOT NULL
	);`
	_, err := cache.db.ExecContext(ctx, query)
	if err != nil {
		return utils.Wrap(err, "failed to create model cache table")
	}
	return nil
}
