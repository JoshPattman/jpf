package jpf

/*

func NewSQLCache(db *sql.DB) (KVCache, error) {
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

func (cache *sqlCache) GetCachedResponse(msgs []Message) (bool, []Message, Message, error) {
	h := HashMessages(msgs)
	row := cache.db.QueryRow(`SELECT resp FROM model_cache WHERE hash=?;`, h)
	blob := []byte{}
	err := row.Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, Message{}, nil
	}
	var outputs []Message
	err = gob.NewDecoder(bytes.NewBuffer(blob)).Decode(&outputs)
	if err != nil {
		return false, nil, Message{}, err
	}
	if len(outputs) == 0 {
		return false, nil, Message{}, errors.New("cached messages have 0 length")
	}
	return true, outputs[:len(outputs)-1], outputs[len(outputs)-1], nil
}

func (cache *sqlCache) SetCachedResponse(inputs []Message, aux []Message, out Message) error {
	h := HashMessages(inputs)
	blob := bytes.NewBuffer(nil)
	err := gob.NewEncoder(blob).Encode(append(aux, out))
	if err != nil {
		return err
	}
	_, err = cache.db.Exec(`INSERT INTO model_cache (hash, resp) VALUES (?, ?) ON CONFLICT(hash) DO UPDATE SET resp = excluded.resp;`, h, blob.Bytes())
	if err != nil {
		return err
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
		return err
	}

	// New table for embeddings
	query = `
	CREATE TABLE IF NOT EXISTS embed_cache (
		text TEXT PRIMARY KEY,
		embedding BLOB NOT NULL
	);`
	_, err = cache.db.Exec(query)
	return err
}

// ===== Implementation of EmbedderResponseCache =====

func (cache *sqlCache) GetCachedEmbedding(input string) (bool, []float64, error) {
	row := cache.db.QueryRow(`SELECT embedding FROM embed_cache WHERE text=?;`, input)
	blob := []byte{}
	err := row.Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	var embedding []float64
	err = gob.NewDecoder(bytes.NewBuffer(blob)).Decode(&embedding)
	if err != nil {
		return false, nil, err
	}

	return true, embedding, nil
}

func (cache *sqlCache) SetCachedEmbedding(input string, embedding []float64) error {
	blob := bytes.NewBuffer(nil)
	err := gob.NewEncoder(blob).Encode(embedding)
	if err != nil {
		return err
	}

	_, err = cache.db.Exec(`
		INSERT INTO embed_cache (text, embedding)
		VALUES (?, ?)
		ON CONFLICT(text) DO UPDATE SET embedding = excluded.embedding;
	`, input, blob.Bytes())
	return err
}
*/
