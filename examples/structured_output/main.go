package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/encoders"
	"github.com/JoshPattman/jpf/models"
	"github.com/JoshPattman/jpf/parsers"
	"github.com/JoshPattman/jpf/pipelines"
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
		person, _, err := personQ.Call(context.Background(), q)
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
		animal, _, err := animalQ.Call(context.Background(), q)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s   >   %s (%s), mammal=%v\n", q, animal.Name, animal.Family, animal.IsMammal)
	}
}

// Builds a Pipeline that answers a question with a typed struct.
// Uses OpenAI gpt 4o, with 5 retries on API failiure.
// IMO this is not as powerful a pattern as the building structs in the other examples,
// but I have added this here to show that it can be simplified.
func BuildStructuredQuerier[T any]() (jpf.Pipeline[string, T], error) {
	var example T
	schema, err := getSchema(example)
	if err != nil {
		return nil, errors.Join(errors.New("failed to create schema"), err)
	}
	model := models.NewRemote(
		models.OpenAI,
		"gpt-4o",
		os.Getenv("OPENAI_KEY"),
		models.WithJSONSchema(schema),
	)
	model = models.Retry(model, 5)
	enc := encoders.NewFixed("Answer the users question in a json format.")
	dec := parsers.NewJson[T]()
	return pipelines.NewOneShot(enc, dec, nil, model), nil
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
