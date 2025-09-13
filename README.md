[![Go Report Card](https://goreportcard.com/badge/github.com/JoshPattman/jpf)](https://goreportcard.com/report/github.com/JoshPattman/jpf)
[![Go Ref](https://pkg.go.dev/static/frontend/badge/badge.svg)](https://pkg.go.dev/github.com/JoshPattman/jpf)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

# `jpf` - A Lightweight Framework for AI-Powered Applications

jpf is a Go library for building lightweight AI-powered applications. It provides essential building blocks, including model construction, embedding generation, and robust LLM interaction interfaces, enabling you to craft custom solutions without the bloat.

jpf is aimed at using AI as a tool - not as a chatbot (this is not to say you cannot use it to make a chatbot, however there is no framework provided for this yet). It focusses on adding AI features locally, as opposed to relying too heavily on external APIs - this makes the package particularly flexible when switching models or providers.

## Features

- **Retry and Feedback Handling**: Resilient mechanisms for retrying tasks and incorporating feedback into interactions.
- **Embedding Utilities**: Generate and manipulate vector embeddings for tasks like similarity, search, or clustering.
- **Customizable Models**: Seamlessly integrate LLMs, including reasoning chains and hybrid models.
- **Token Usage Tracking**: Stay informed of API token consumption for cost-effective development.
- **Easy-to-use Caching**: Reduce the calls made to models by composing a caching layer onto an existing model.
- **Out-of-the-box Logging**: Simply add logging messages to your models, helping yuo track down issues.

## Installation

Install jpf in your Go project via:

```bash
go get github.com/JoshPattman/jpf
```

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.

## Contributing

Contributions are welcome! Open an issue or submit a pull request on GitHub.

## FAQ
- Will streaming (token-by-token) ever be supported?
    - No. This framework is designed to be more of a back-end tool, and character streaming would add extra complexity that most applications of this package would not benefit from (in my opinion).
- Are there any pre-built formatters / parsers?
    - There are a few built in implementations, however the aim of this package is to create the framework, not the functionality.
    - If you have any ideas of useful functions, feel free to put them on an issue, and if enough arise, I can make a new repo for these.
- Where are the agents?
    - This package is tries to simplify single calls to LLMs, which is a level below what agents do.
    - I have plans to build an agent framework on top of this package, but I would like to build a strong foundation first.
- Why does this not support MCP tools on the OpenAI API / Tool calling / Other advanced API feature?
    - The aim of this package is to put the advanced stuff, like using tools, to you to figure out. IMO this allows you to do cooler, more flexible things (like a tree of agents).
    - Also, to a degree tool calls / MCP tools lock you in to one API or another, more than just using the chat completions endpoint.
    - I might consider adding them in the future, but for now I think that implementing your own tool calling is best.
    - As a rule of thumb, I will add API features that fiddle with the log probs (e.g. structured output, temperature, top p, ...) but I will not add somthing if a model could not acheive the same result with perfect prompting.

## Author

Developed by Josh Pattman. Learn more at [GitHub](https://github.com/JoshPattman/jpf).
