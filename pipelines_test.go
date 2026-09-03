package jpf

import "testing"

func TestPipelineResponseOnlyUsage(t *testing.T) {
	r := PipelineResponse[string]{Result: "hi", Usage: Usage{InputTokens: 5}}
	got := r.OnlyUsage()
	if got.Result != "" || got.Usage.InputTokens != 5 {
		t.Fatalf("got %+v", got)
	}
}

func TestPipelineResponseIncludingUsage(t *testing.T) {
	r := PipelineResponse[string]{Result: "hi", Usage: Usage{InputTokens: 5}}
	got := r.IncludingUsage(Usage{InputTokens: 2})
	if got.Result != "hi" || got.Usage.InputTokens != 7 {
		t.Fatalf("got %+v", got)
	}
}
