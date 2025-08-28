package jpf

import "testing"

func TestConstructAllEmbedders(t *testing.T) {
	NewOpenAIEmbedder("abc", "def", WithURL{X: "ghi"})
}
