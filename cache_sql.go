package jpf

import (
	"database/sql"
	"errors"
)

// NewSQLCache creates a new SQL-backed key-value cache.
func NewSQLCache(db *sql.DB) (Cache, error) {
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

// Set stores a value in the SQL cache under the given key.
// If the key already exists, the value is updated.
func (cache *sqlCache) Set(key string, data []byte) error {
	_, err := cache.db.Exec(`
		INSERT INTO kv_cache (key, value)
		VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value;
	`, key, data)
	return err
}

// Get retrieves a value from the SQL cache by key.
// Returns ErrNoCache if the key does not exist.
func (cache *sqlCache) Get(key string) ([]byte, error) {
	row := cache.db.QueryRow(`SELECT value FROM kv_cache WHERE key=?;`, key)
	var data []byte
	err := row.Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoCache
	}
	if err != nil {
		return nil, err
	}
	return data, nil
}

// setupDB ensures that the underlying table exists.
func (cache *sqlCache) setupDB() error {
	query := `
	CREATE TABLE IF NOT EXISTS kv_cache (
		key TEXT PRIMARY KEY,
		value BLOB NOT NULL
	);`
	_, err := cache.db.Exec(query)
	return err
}
