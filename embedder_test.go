package jpf

import "testing"

func TestConstructAllEmbedders(t *testing.T) {
	var builder EmbedderBuilder = nil
	builder = BuildOpenAIEmbedder("abc", "def").WithURL("ghi")
	_, err := builder.New()
	if err != nil {
		t.Fatal(err)
	}
}
