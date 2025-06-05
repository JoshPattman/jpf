package jpf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type OpenAIEmbedderBuilder struct {
	embedder *openAIEmbedder
}

func BuildOpenAIEmbedder(key, model string) *OpenAIEmbedderBuilder {
	return &OpenAIEmbedderBuilder{
		embedder: &openAIEmbedder{
			key:   key,
			model: model,
			url:   embeddingOpenAIUrl,
		},
	}
}

func (b *OpenAIEmbedderBuilder) Validate() (Embedder, error) {
	return b.embedder, nil
}

func (b *OpenAIEmbedderBuilder) WithURL(url string) *OpenAIEmbedderBuilder {
	b.embedder.url = url
	return b
}

const embeddingOpenAIUrl = "https://api.openai.com/v1/embeddings"

type openAIEmbedder struct {
	key   string
	model string
	url   string
}

func (o *openAIEmbedder) Embed(text string) ([]float64, error) {
	bodyMap := map[string]any{
		"input": text,
		"model": o.model,
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", o.url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", o.key))
	req.Header.Add("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	respTyped := struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}{}
	err = json.Unmarshal(respBody, &respTyped)
	if err != nil || len(respTyped.Data) != 1 || len(respTyped.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("failed to parse response (%v): %s", err.Error(), string(respBody))
	}
	return respTyped.Data[0].Embedding, nil
}
