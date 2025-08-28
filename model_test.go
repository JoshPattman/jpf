package jpf

import (
	"os"
	"testing"
	"time"
)

func TestConstructAllModels(t *testing.T) {
	model := NewOpenAIModel("abc", "123", WithHTTPHeader{K: "A", V: "B"}, WithReasoningEffort{X: HighReasoning}, WithTemperature{X: 0.5}, WithURL{X: "abc.com"})
	model = NewCachedModel(model, NewInMemoryCache())
	model = NewConcurrentLimitedModel(model, NewOneConcurrentLimiter())
	model = NewFakeReasoningModel(model, model, WithReasoningPrompt{X: "Reason please"})
	model = NewLoggingModel(model, NewJsonModelLogger(os.Stdout))
	model = NewRetryModel(model, WithDelay{X: time.Second}, WithRetries{X: 10})
	NewSystemReasonModel(model, WithReasoningPrefix{X: "Resoning: "})
}
