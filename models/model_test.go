package models

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
	"golang.org/x/sync/semaphore"
)

func NewMockLogger() *mockLogger {
	return &mockLogger{}
}

type mockLogger struct {
	infos []jpf.ModelLoggingInfo
}

func (m *mockLogger) ModelLog(info jpf.ModelLoggingInfo) error {
	m.infos = append(m.infos, info)
	return nil
}

func (m *mockLogger) Infos() []jpf.ModelLoggingInfo {
	return slices.Clone(m.infos)
}

func TestConstructOtherModels(t *testing.T) {
	model := NewAPIModel(OpenAI, "abc", "123", WithHeader("A", "B"), WithTemperature(0.5))
	model = LimitConcurrency(model, semaphore.NewWeighted(1))
	model = TwoStageReason(model, model, WithReasoningPrompt("Reason please"))
	model = Log(model, NewMockLogger())
	model = Retry(model, 10, WithDelay(time.Second))
	RetryChain([]jpf.Model{model, model})
}

func TestCachedModel(t *testing.T) {
	var model jpf.Model = &utils.TestingModel{Responses: map[string][]string{
		"hello": {"hi", "bye"},
	}}
	model = Cache(model, newRamCache())
	for i := range 5 {
		resp1, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp1.PrimaryMessage.Content != "hi" {
			t.Fatalf("expected 'hi' but got '%v' on iteration %v", resp1.PrimaryMessage.Content, i)
		}
	}
}

func TestLoggingModel(t *testing.T) {
	responseSeq := []string{"hi", "bye", "hi again"}
	var model jpf.Model = &utils.TestingModel{Responses: map[string][]string{
		"hello": responseSeq,
	}}
	logger := NewMockLogger()
	model = Log(model, logger)
	for range 3 {
		_, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
	}

	if len(logger.Infos()) != 3 {
		t.Fatalf("expected 3 logs got %d", len(logger.Infos()))
	}
	for i := range responseSeq {
		if logger.Infos()[i].ResponseFinalMessage.Content != responseSeq[i] {
			t.Fatalf("expected '%s' got '%s'", logger.Infos()[i].ResponseFinalMessage.Content, responseSeq[i])
		}
	}
}

func TestRetryModel(t *testing.T) {
	var model jpf.Model = &utils.TestingModel{
		Responses: map[string][]string{
			"hello": {"hi", "bye", "hi again"},
		},
		NFails: 3,
	}
	model = Retry(model, 3)
	resp, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PrimaryMessage.Content != "hi" {
		t.Fatalf("expected 'hi' but got '%v'", resp.PrimaryMessage.Content)
	}
}

func TestRetryChainModel(t *testing.T) {
	t.Run("first model succeeds", func(t *testing.T) {
		model := RetryChain([]jpf.Model{
			&utils.TestingModel{
				Responses: map[string][]string{
					"hello": {"first response"},
				},
			},
			&utils.TestingModel{
				Responses: map[string][]string{
					"hello": {"second response"},
				},
			},
		})
		resp, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "first response" {
			t.Fatalf("expected 'first response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})

	t.Run("first model fails, second succeeds", func(t *testing.T) {
		model := RetryChain([]jpf.Model{
			&utils.TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
			&utils.TestingModel{
				Responses: map[string][]string{
					"hello": {"second response"},
				},
			},
		})
		resp, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "second response" {
			t.Fatalf("expected 'second response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})

	t.Run("all models fail", func(t *testing.T) {
		model := RetryChain([]jpf.Model{
			&utils.TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
			&utils.TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
			&utils.TestingModel{
				Responses: map[string][]string{},
				NFails:    1,
			},
		})
		_, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err == nil {
			t.Fatal("expected error but got none")
		}
		// Check that error mentions all models failed
		errMsg := err.Error()
		if !contains(errMsg, "all 3 models") {
			t.Fatalf("expected error to mention all 3 models, got: %v", errMsg)
		}
	})

	t.Run("third model succeeds", func(t *testing.T) {
		model := RetryChain([]jpf.Model{
			&utils.TestingModel{NFails: 1},
			&utils.TestingModel{NFails: 1},
			&utils.TestingModel{
				Responses: map[string][]string{
					"hello": {"third response"},
				},
			},
		})
		resp, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatal(err)
		}
		if resp.PrimaryMessage.Content != "third response" {
			t.Fatalf("expected 'third response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})
}

func TestTimeoutModel(t *testing.T) {
	t.Run("timeout triggers on slow model", func(t *testing.T) {
		// Create a slow model that takes 200ms
		slowModel := &utils.SlowTestingModel{
			Delay: 200 * time.Millisecond,
			Response: jpf.ModelResponse{
				PrimaryMessage: jpf.Message{Role: jpf.AssistantRole, Content: "response"},
			},
		}

		// Wrap with 50ms timeout
		model := Timeout(slowModel, 50*time.Millisecond)

		start := time.Now()
		_, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		elapsed := time.Since(start)

		// Should fail with context deadline exceeded
		if err == nil {
			t.Fatal("expected timeout error but got none")
		}
		if !contains(err.Error(), "context deadline exceeded") {
			t.Fatalf("expected 'context deadline exceeded' error, got: %v", err)
		}

		// Should have timed out around 50ms, not 200ms
		if elapsed > 100*time.Millisecond {
			t.Fatalf("timeout took too long: %v (expected ~50ms)", elapsed)
		}
	})

	t.Run("succeeds when operation is fast enough", func(t *testing.T) {
		// Create a fast model that takes 10ms
		fastModel := &utils.SlowTestingModel{
			Delay: 10 * time.Millisecond,
			Response: jpf.ModelResponse{
				PrimaryMessage: jpf.Message{Role: jpf.AssistantRole, Content: "fast response"},
			},
		}

		// Wrap with 100ms timeout (plenty of time)
		model := Timeout(fastModel, 100*time.Millisecond)

		resp, err := model.Respond(context.Background(), []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.PrimaryMessage.Content != "fast response" {
			t.Fatalf("expected 'fast response' but got '%v'", resp.PrimaryMessage.Content)
		}
	})

	t.Run("parent context timeout takes precedence when shorter", func(t *testing.T) {
		// Create a slow model
		slowModel := &utils.SlowTestingModel{
			Delay: 200 * time.Millisecond,
			Response: jpf.ModelResponse{
				PrimaryMessage: jpf.Message{Role: jpf.AssistantRole, Content: "response"},
			},
		}

		// Model configured with 100ms timeout
		model := Timeout(slowModel, 100*time.Millisecond)

		// But parent context has 30ms timeout (shorter)
		parentCtx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := model.Respond(parentCtx, []jpf.Message{{Role: jpf.SystemRole, Content: "hello"}})
		elapsed := time.Since(start)

		// Should fail with context deadline exceeded
		if err == nil {
			t.Fatal("expected timeout error but got none")
		}

		// Should have timed out around 30ms (parent timeout), not 100ms
		if elapsed > 60*time.Millisecond {
			t.Fatalf("timeout took too long: %v (expected ~30ms from parent)", elapsed)
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newRamCache() jpf.ModelResponseCache {
	return &inMemoryCache{
		Resps: make(map[string]memoryCachePacket),
	}
}

type memoryCachePacket struct {
	Aux   []jpf.Message
	Final jpf.Message
}

type inMemoryCache struct {
	Resps map[string]memoryCachePacket
}

func (i *inMemoryCache) GetCachedResponse(ctx context.Context, salt string, msgs []jpf.Message) (bool, []jpf.Message, jpf.Message, error) {
	msgsHash := msgs[0].Content
	if cp, ok := i.Resps[msgsHash]; ok {
		return true, cp.Aux, cp.Final, nil
	}
	return false, nil, jpf.Message{}, nil
}

func (i *inMemoryCache) SetCachedResponse(ctx context.Context, salt string, inputs []jpf.Message, aux []jpf.Message, out jpf.Message) error {
	msgsHash := inputs[0].Content
	i.Resps[msgsHash] = memoryCachePacket{
		Aux:   aux,
		Final: out,
	}
	return nil
}
