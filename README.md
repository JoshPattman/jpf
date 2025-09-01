[![Go Report Card](https://goreportcard.com/badge/github.com/JoshPattman/jpf)](https://goreportcard.com/report/github.com/JoshPattman/jpf)
[![Go Ref](https://pkg.go.dev/static/frontend/badge/badge.svg)](https://pkg.go.dev/github.com/JoshPattman/jpf)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

# jpf: A Lightweight Framework for AI-Powered Applications

jpf is a Go library for building lightweight AI-powered applications. It provides essential building blocks, including model construction, embedding generation, and robust LLM interaction interfaces, enabling you to craft custom solutions without the bloat. It is aimed at using AI as a tool - not as a chatbot.

## Features

- **Flexible Orchestration**: Design iterative workflows for reasoning, task automation, or application backends.
- **Retry and Feedback Handling**: Resilient mechanisms for retrying tasks and incorporating feedback into interactions.
- **Embedding Utilities**: Generate and manipulate vector embeddings for tasks like similarity, search, or clustering.
- **Customizable Models**: Seamlessly integrate LLMs, including reasoning chains and hybrid models.
- **Token Usage Tracking**: Stay informed of API token consumption for cost-effective development.
- **Easy-to-use Caching**: Reduce the calls made to models by composing a caching layer onto an existing model.

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
    - There are a few buildt in implementations, however the aim of this package is to create the framework, not the functionality.
    - If you have any ideas of useful functions, feel free to put them on an issue, and if enough arise, I can make a new repo for these.
- Where are the agents?
    - I removed the agent interface recently as I think it was far too restrictuve.
    - I would like to instead get the core building blocks ironed out before moving on to coming up with an agent interface.

## Author

Developed by Josh Pattman. Learn more at [GitHub](https://github.com/JoshPattman/jpf).