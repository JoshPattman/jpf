<p align="center">
  <img src="res/banner.webp" width="100%">
</p>

[![Go Report Card](https://goreportcard.com/badge/github.com/JoshPattman/jpf)](https://goreportcard.com/report/github.com/JoshPattman/jpf)
[![Go Ref](https://pkg.go.dev/static/frontend/badge/badge.svg)](https://pkg.go.dev/github.com/JoshPattman/jpf)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Providing essential building blocks and robust LLM interaction interfaces, **jpf** enables you to craft custom AI solutions without the bloat.

## Features

- **Retry and Feedback Handling**: Resilient mechanisms for retrying tasks and incorporating feedback into interactions.
- **Customizable Models**: Seamlessly integrate LLMs from multiple providers using unified interfaces.
- **Token Usage Tracking**: Stay informed of API token consumption for cost-effective development.
- **Stream Responses**: Keep your users engaged with responses that are streamed back as they are generated.
- **Easy-to-use Caching**: Reduce the calls made to models by composing a caching layer onto an existing model.
- **Out-of-the-box Logging**: Simply add logging messages to your models, helping you track down issues.
- **Industry Standard Context Management**: All potentially slow interfaces support Go's context.Context for timeouts and cancellation.
- **Rate Limit Management**: Compose models together to set local rate limits to prevent API errors.
- **MIT License**: Use the code for anything, anywhere, for free.

## Installation

Install jpf in your Go project via:

```bash
go get github.com/JoshPattman/jpf
```

Learn more about JPF in the [Core Concepts](#core-concepts) section.

## Quickstart

There are multiple examples available in the [examples](https://github.com/JoshPattman/jpf/examples) directory.

### Build a model
- A model is capable of responding to a set of messages.
- Models are built through composition, adding functionality that runs on your machine.

```go
func BuildModel() jpf.Model {
	// Create a new gpt-4o model attached to the OpenAI API.
	model := models.NewRemote(
		models.OpenAI, // Defines the API format and the default URL (URL can be overriden)
		"gpt-4o", // Model name on API
		os.Getenv("OPENAI_KEY"), // API key
		models.WithTemperature(0.5) // Optional params - many more are supported
	)
	// Locally rate limit the model calls to 1 every 5 seconds
	model = models.RateLimit(model, rate.NewLimiter(rate.Every(time.Second*5), 1))
	// Make the model retry non-200 requests up to 5 times
	model = models.Retry(model, 5)
	// Cache model requests in memory - file and database are also supported
	cache := caches.NewRAM()
	model = models.Cache(model, cache)
	return model
}
```

### Build a pipeline
- A pipeline is a wrapper around a model that takes and returns structured data.
- Pipelines may retry using various strategies when a validation error (attempting to parse the output) occurs.
```go
// Define the structured data to provide to the pipeline
type TaskInput struct {
	Name string
}
// Define the structured data to read from the pipeline
type TaskOutput struct {
	IsCelebrity bool `json:"is_celebrity"`
}

// Define a custom validator that will not accpet that santa is not a celebrity
type CustomValidator struct{}

func (c *CustomValidator) ValidateParsedResponse(in TaskInput, out TaskOutput) error {
	if strings.ToLower(in.Name) == "santa" && !out.IsCelebrity {
		return errors.New("santa is a celebrity")
	}
	return nil
}

func BuildPipeline(model jpf.Model) jpf.Pipeline[TaskInput, TaskOutput] {
	// Encode the data to system/user prompt pair, where both are a text/template
	encoder := encoders.NewTemplate[TaskInput]("The user will give you a name. Respond with a json object with a single key, 'is_celebrity'.", "{{ .Name }}")
	// Parse the output message into a struct using json
	parser := parsers.NewJson[TaskOutput]()
	// Only provide the text between { and } to the json parser - cut off extra stuff like backticks
	parser = parsers.SubstringJsonObject(parser)
	// When retrying, will provide fedback by simply formatting the error
	feedback := feedbacks.NewErrString()
	// Create a pipeline that retries up to 5 times on parsing or validation errors, providing feedback as developer
	return pipelines.NewFeedbackRetry(
		encoder,
		parser,
		&CustomValidator{}, // This is allowed to be nil if no further validation is required
		feedback,
		model,
		jpf.DeveloperRole,
		5,
	)
}
```

### Use the pipeline
```go
func IsCelebrity(name string) (bool, error) {
	// Realistically in production code, you would not build the models here,
	// instead you would inject them (or at least inject the builders),
	// as this allows for higher testability and customisability.
	model := BuildModel()
	pipeline := BuildPipeline(model)
	// Calling a pipeline gives result, usage, and error
	result, usage, err := pipeline.Call(context.Background(), TaskInput{name})
	if err != nil {
		return false, err
	}
	fmt.Println(usage)
	return result.IsCelebrity, nil
}
```


## FAQ
- I want to change my model's temperature/structured output/output tokens/... after I have built it!
	- The intention is to provide functions that need to use an LLM with a builder function instead of a built object. This way, you can use the builder function multiple times with different parameters.
	- Take a look at the examples to see this concept.
	- This design decision was made as it prevents you from injecting unnecessary LLM-related data into business logic.
- Where are the agents?
	- Agents are built on top of LLMs, but this package is designed for LLM handling, so it lives at the level below agents.
	- Take a look at [JChat](https://github.com/JoshPattman/agent/cmd/jchat) or [react](https://github.com/JoshPattman/react) to see how you can build an agent on top of JPF.
- Why does this not support MCP tools on the OpenAI API / Tool calling / Other advanced API features?
	- Relying on API features like tool calling, MCP tools, or vector stores is not ideal for two reasons: (a) it makes it harder to move between API/model providers (b) it gives you less flexibility and control.
	- These features are not particularly hard to add locally, so you should aim to do so to ensure your application is as robust as possible to API change.

## Author

Developed by Josh Pattman. Learn more at [GitHub](https://github.com/JoshPattman/jpf).
