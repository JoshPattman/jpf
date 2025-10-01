package jpf

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"errors"
)

func NewSQLCache(db *sql.DB) (ModelResponseCache, error) {
	c := &sqlCache{
		db: db,
	}
	err := c.setupDB()
	if err != nil {
		return nil, err
	}
	return c, nil
}

type sqlCache struct {
	db *sql.DB
}

type sqlModelCachePayload struct {
	Aux []Message
}

func (cache *sqlCache) GetCachedResponse(salt string, msgs []Message) (bool, []Message, Message, error) {
	h := HashMessages(salt, msgs)
	row := cache.db.QueryRow(`SELECT resp FROM model_cache WHERE hash=?;`, h)
	blob := []byte{}
	err := row.Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, Message{}, nil
	} else if err != nil {
		return false, nil, Message{}, wrap(err, "failed to query database")
	}
	var outputs []Message
	err = gob.NewDecoder(bytes.NewBuffer(blob)).Decode(&outputs)
	if err != nil {
		return false, nil, Message{}, wrap(err, "failed to decode cached data")
	}
	if len(outputs) == 0 {
		return false, nil, Message{}, errors.New("cached messages have 0 length")
	}
	return true, outputs[:len(outputs)-1], outputs[len(outputs)-1], nil
}

func (cache *sqlCache) SetCachedResponse(salt string, inputs []Message, aux []Message, out Message) error {
	h := HashMessages(salt, inputs)
	blob := bytes.NewBuffer(nil)
	err := gob.NewEncoder(blob).Encode(append(aux, out))
	if err != nil {
		return wrap(err, "failed to encode messages to binary data")
	}
	_, err = cache.db.Exec(`INSERT INTO model_cache (hash, resp) VALUES (?, ?) ON CONFLICT(hash) DO UPDATE SET resp = excluded.resp;`, h, blob.Bytes())
	if err != nil {
		return wrap(err, "failed to execute database insert")
	}
	return nil
}

func (cache *sqlCache) setupDB() error {
	query := `
	CREATE TABLE IF NOT EXISTS model_cache (
		hash TEXT PRIMARY KEY,
		resp BLOB NOT NULL
	);`
	_, err := cache.db.Exec(query)
	if err != nil {
		return wrap(err, "failed to create model cache table")
	}
	return nil
}
