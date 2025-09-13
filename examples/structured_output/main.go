package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/JoshPattman/jpf"
	"github.com/invopop/jsonschema"
)

type PersonResponse struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Age       int    `json:"age"`
}

type AnimalResponse struct {
	Name     string `json:"name"`
	Family   string `json:"family"`
	IsMammal bool   `json:"is_mammal"`
}

func main() {
	// Ask some questions about people
	personQ, err := BuildStructuredQuerier[PersonResponse]()
	if err != nil {
		panic(err)
	}
	questions := []string{
		"Who wrote the original transformers paper?",
		"Who is the president of the US?",
	}
	for _, q := range questions {
		person, _, err := personQ.Call(q)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s   >   %s %s, %d years old\n", q, person.FirstName, person.LastName, person.Age)
	}

	// Ask some questions about animals
	animalQ, err := BuildStructuredQuerier[AnimalResponse]()
	if err != nil {
		panic(err)
	}
	questions = []string{
		"What is mans best friend?",
		"What is the fastest bird?",
	}
	for _, q := range questions {
		animal, _, err := animalQ.Call(q)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s   >   %s (%s), mammal=%v\n", q, animal.Name, animal.Family, animal.IsMammal)
	}
}

// Builds a MapFunc that answers a question with a typed struct.
// Uses OpenAI gpt 4o, with 5 retries on API failiure.
func BuildStructuredQuerier[T any]() (jpf.MapFunc[string, T], error) {
	var example T
	schema, err := getSchema(example)
	if err != nil {
		return nil, errors.Join(errors.New("failed to create schema"), err)
	}
	model := jpf.NewOpenAIModel(
		os.Getenv("OPENAI_KEY"),
		"gpt-4o",
		jpf.WithJsonSchema{X: schema},
	)
	model = jpf.NewRetryModel(model, 5)
	enc := jpf.NewRawStringMessageEncoder("Answer the users question in a json format.")
	dec := jpf.NewJsonResponseDecoder[T]()
	return jpf.NewOneShotMapFunc(enc, dec, model), nil
}

// Generates a json schema from an example struct.
// For now this is not in core jpf as it would add dependancies,
// and I am unsure if this is the best way to generate json schemas yet.
func getSchema(obj any) (map[string]any, error) {
	r := &jsonschema.Reflector{
		BaseSchemaID:   "Anonymous",
		Anonymous:      true,
		DoNotReference: true,
	}
	s := r.Reflect(obj)
	schemaBs, err := s.MarshalJSON()
	if err != nil {
		return nil, err
	}
	schema := make(map[string]any)
	err = json.Unmarshal(schemaBs, &schema)
	if err != nil {
		return nil, err
	}
	return schema, nil
}
