package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/JoshPattman/jpf"
)

//go:embed system.md
var system string

type TemplateData struct {
	Language string
	Pointers []string
	Code     string
}

func main() {
	inputFile := flag.String("i", "", "The input file to use")
	outputFile := flag.String("o", "", "The output file to use")
	targetLang := flag.String("l", "Python", "Target language for the rewrite")
	pointers := flag.String("p", "You may change the overall structure;Make sure to keep the functionality the same", "Semicolon separated list of pointers")
	flag.Parse()

	model := jpf.NewOpenAIModel(os.Getenv("OPENAI_KEY"), "gpt-4o-mini")
	formatter := jpf.NewTemplateMessageEncoder[TemplateData](system, "{{.Code}}")
	parser := jpf.NewRawStringResponseDecoder()
	mapFunc := jpf.NewOneShotMapFunc(formatter, parser, model)

	f, err := os.Open(*inputFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Println(err)
		return
	}
	rewritten, _, err := mapFunc.Call(TemplateData{
		Language: *targetLang,
		Pointers: strings.Split(*pointers, ";"),
		Code:     string(data),
	})
	f2, err := os.Create(*outputFile)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f2.Close()
	fmt.Fprint(f2, rewritten)
}
