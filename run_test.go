package jpf

import (
	"bytes"
	"fmt"
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	oaik := os.Getenv("OPENAI_KEY")
	model := NewOpenAIModel(oaik, "gpt-4o-mini")
	model = NewCachedModel(model, NewInMemoryCache())
	buf := bytes.NewBuffer(nil)
	model = NewLoggingModel(model, NewJsonModelLogger(buf))

	type Data struct {
		Name     string
		Pointers []string
	}

	type Response struct {
		Greeting string
	}

	formatter := NewTemplateMessageEncoder[Data](
		`You should greet {{.Name}}. Be nice. Return your greeting as a json object, with one key, 'greeting'.
{{- if .Pointers }}
Here are the pointers:
{{- range .Pointers }}
- {{ . }}
{{- end }}
{{- end }}
`, "")
	parser := NewJsonResponseDecoder[Response]()
	mf := NewOneShotMapFunc(formatter, parser, model)

	fmt.Println(mf.Call(Data{Name: "Josh", Pointers: []string{"In the style of a pirate"}}))
	fmt.Println(buf.String())
}
