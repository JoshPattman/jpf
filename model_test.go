package jpf

import (
	"os"
	"testing"
)

func TestConstructAllModels(t *testing.T) {
	cache := NewInMemoryCache()
	limiter := NewOneConcurrentLimiter()
	var builder ModelBuilder
	builder = BuildOpenAIModel("abc", "123", true)
	builder = BuildCachedModel(builder, cache)
	builder = BuildConcurrentLimitedModel(builder, limiter)
	builder = BuildFakeReasoningModel(builder, builder)
	builder = BuildLoggingModel(builder, os.Stdout)
	builder = BuildRetryModel(builder).WithMaxRetries(10)
	builder = BuildSystemReasonModel(builder)
	_, err := builder.New()
	if err != nil {
		t.Fatalf("expecting no error from constructing models, got %v", err)
	}
}
