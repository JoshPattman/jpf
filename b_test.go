package jpf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
)

type EMCase struct {
	ID       string
	Build    func() EmbedCaller
	Expected []float64
	Server   func() func()
}

func (testCase EMCase) Name() string { return testCase.ID }

func (testCase EMCase) Test() error {
	if testCase.Server != nil {
		defer testCase.Server()()
	}
	embedder := testCase.Build()
	result, err := embedder.Call("abcdefg")
	if err != nil {
		return errors.Join(errors.New("expected embedder to not err, got error"), err)
	}
	if len(result) != len(testCase.Expected) {
		return fmt.Errorf("expected embedding of length %v but got %v", len(testCase.Expected), result)
	}
	for i := range result {
		if result[i] != testCase.Expected[i] {
			return fmt.Errorf("expected embedding %v to be %v but got %v", i, testCase.Expected[i], result[i])
		}
	}
	return nil
}

var EMCases = []TestCase{
	EMCase{
		ID: "openai",
		Build: func() EmbedCaller {
			return NewOpenAIEmbedCaller("key", "model", WithURL{"http://localhost:1234/embedding"})
		},
		Expected: []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		Server: func() func() {
			r := http.NewServeMux()
			r.HandleFunc("/embedding", func(w http.ResponseWriter, r *http.Request) {
				input := struct {
					Input string `json:"input"`
					Model string `json:"model"`
				}{}
				err := json.NewDecoder(r.Body).Decode(&input)
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if input.Input != "abcdefg" || input.Model != "model" {
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				emb := make([]float64, 10)
				emb[9] = 1
				result := map[string]any{
					"data": []map[string]any{
						{"embedding": emb},
					},
				}
				json.NewEncoder(w).Encode(result)
			})
			srv := &http.Server{Addr: ":1234", Handler: r}
			go srv.ListenAndServe()
			return func() { srv.Shutdown(context.TODO()) }
		},
	},
}

func TestConstructAllEmbedders(t *testing.T) {
	RunTests(t, EMCases)
}
