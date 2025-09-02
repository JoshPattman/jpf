package jpf

import (
	"bytes"
	"image"
	"image/color"
	"math"
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

	hello, usage, err := mf.Call(Data{Name: "Josh", Pointers: []string{"In the style of a pirate"}})
	if err != nil {
		t.Fatal(err)
	}
	if hello.Greeting == "" || usage.InputTokens*usage.OutputTokens == 0 {
		t.Fatal("Invalid response")
	}
	t.Log(hello, usage)
	debug := buf.String()
	if debug == "" {
		t.Fatal("Debug was empty")
	}
	t.Log(debug)

	type Response2 struct {
		Country string
	}

	parser2 := NewJsonResponseDecoder[Response2]()
	_, resp, _, err := model.Respond([]Message{
		{
			Role:    SystemRole,
			Content: "Tell the user what flag they have uploaded. Respond with a json object with a single key, `country` that is a string with the country name in lowercase",
		},
		{
			Role:    UserRole,
			Content: "Flag:",
			Images:  []ImageAttachment{{NewRedCircleImage(256)}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := parser2.ParseResponseText(resp.Content)
	if err != nil {
		t.Fatal(err)
	}
	if result.Country != "japan" {
		t.Fatal("incorrect country", result.Country)
	}
}

type redCircleOnWhite struct {
	size   int
	radius float64
	center image.Point
}

// ColorModel implements image.Image.
func (rc *redCircleOnWhite) ColorModel() color.Model {
	return color.RGBAModel
}

// Bounds implements image.Image.
func (rc *redCircleOnWhite) Bounds() image.Rectangle {
	return image.Rect(0, 0, rc.size, rc.size)
}

// At lazily generates the pixel color.
func (rc *redCircleOnWhite) At(x, y int) color.Color {
	// White background
	c := color.RGBA{255, 255, 255, 255}

	// Compute distance from center
	dx := float64(x - rc.center.X)
	dy := float64(y - rc.center.Y)
	if math.Sqrt(dx*dx+dy*dy) <= rc.radius {
		// Inside the circle → red
		c = color.RGBA{255, 0, 0, 255}
	}
	return c
}

// NewRedCircleImage creates the lazy image.
func NewRedCircleImage(size int) image.Image {
	return &redCircleOnWhite{
		size:   size,
		radius: float64(size) * 0.3, // circle radius = 30% of size
		center: image.Point{X: size / 2, Y: size / 2},
	}
}
