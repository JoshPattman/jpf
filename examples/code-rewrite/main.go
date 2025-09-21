package main

import (
	"database/sql"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/JoshPattman/jpf"
	_ "github.com/mattn/go-sqlite3"
)

//go:embed system.md
var systemPrompt string

// CodeConversionInput is all of the input data provided to a conde conversion task.
type CodeConversionInput struct {
	Language string
	Pointers []string
	Code     string
}

func main() {
	// Handle args
	inputFile := flag.String("i", "", "The input file to use")
	outputFile := flag.String("o", "", "The output file to use")
	targetLang := flag.String("l", "Python", "Target language for the rewrite")
	pointers := flag.String("p", "You may change the overall structure;Make sure to keep the functionality the same", "Semicolon separated list of pointers")
	useGemini := flag.Bool("g", false, "If specified, we will use gemini instead of openai")
	flag.Parse()

	// Open a db and set up persistent
	db, err := sql.Open("sqlite3", "./cache.db")
	if err != nil {
		fmt.Println(err)
		return
	}
	cache, err := jpf.NewSQLCache(db)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Set up our builders
	modelBuilder := &ModelBuilder{
		OpenAIKey:       os.Getenv("OPENAI_KEY"),
		OpenAIModelName: "gpt-4o-mini",
		GeminiKey:       os.Getenv("GEMINI_KEY"),
		GeminiModelName: "gemini-2.0-flash",
		Cache:           cache,
		Retries:         5,
	}
	codeConnvertBuilder := &CodeConvertMFBuilder{
		ModelBuilder: modelBuilder,
		SystemPrompt: systemPrompt,
	}

	// Build the code converter
	codeConverter := codeConnvertBuilder.Build(*useGemini)

	// Read the input file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Rewrite the code
	rewritten, _, err := codeConverter.Call(CodeConversionInput{
		Language: *targetLang,
		Pointers: strings.Split(*pointers, ";"),
		Code:     string(data),
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	// Write the output file
	f2, err := os.Create(*outputFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f2.Close()
	fmt.Fprint(f2, rewritten)
}

// ModelBuilder can build up jpf.Models with various behaviour.
// Some of the data for building models comes from its fields, some is passed in at build time.
// Each build model will share persistent resources (i.e. cache).
type ModelBuilder struct {
	OpenAIKey       string
	OpenAIModelName string
	GeminiModelName string
	GeminiKey       string
	Cache           jpf.ModelResponseCache
	Retries         int
}

func (builder *ModelBuilder) Build(useGemini bool) jpf.Model {
	var model jpf.Model
	if useGemini {
		model = jpf.NewGeminiModel(builder.GeminiKey, builder.GeminiModelName)
	} else {
		model = jpf.NewOpenAIModel(builder.OpenAIModelName, builder.OpenAIKey)
	}
	if builder.Retries > 0 {
		model = jpf.NewRetryModel(model, builder.Retries, jpf.WithDelay{X: 1})
	}
	if builder.Cache != nil {
		model = jpf.NewCachedModel(model, builder.Cache)
	}
	return model
}

// CodeConvertMFBuilder builds a code converting jpf.MapFunc.
// It uses the provided model builder to build the model, and wraps it with an encoder and decoder.
// You may choose to build an openAI or gemini based map func at build time.
type CodeConvertMFBuilder struct {
	ModelBuilder *ModelBuilder
	SystemPrompt string
}

func (builder *CodeConvertMFBuilder) Build(useGemini bool) jpf.MapFunc[CodeConversionInput, string] {
	model := builder.ModelBuilder.Build(useGemini)

	formatter := jpf.NewTemplateMessageEncoder[CodeConversionInput](builder.SystemPrompt, "{{.Code}}")
	parser := jpf.NewRawStringResponseDecoder()
	return jpf.NewOneShotMapFunc(formatter, parser, model)
}
