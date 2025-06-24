[![Go Report Card](https://goreportcard.com/badge/github.com/JoshPattman/jpf)](https://goreportcard.com/report/github.com/JoshPattman/jpf)
[![Go Ref](https://pkg.go.dev/static/frontend/badge/badge.svg)](https://pkg.go.dev/github.com/JoshPattman/jpf)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

# jpf: A Lightweight Framework for AI-Powered Applications

jpf is a Go library for building lightweight AI-powered applications. It provides essential building blocks, including model construction, embedding generation, and robust LLM interaction interfaces, enabling you to craft custom solutions without the bloat.

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

## Usage

Example code is in the `examples` subdirectory. However, a brief overview of the components is as follows:
- `Model`: An interface defining a model that can create a message given a set of other messages. This encompasses both normal and reasoning models. Models can also be wrapped by other models to achieve retry logic, hybrid reasoning, and more.
- `Function`: An interface that defines a stateless typed function that performs one LLM call, and includes logic for formatting and parsing the text responses.
- `RetryFunction`: An extension of the above, but including logic to generate feedback for the LLM upon a failed parse, allowing the LLM call to be run in a loop until valid.
- `Embedder`: A string to vector embbedding interface.

## License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.

## Contributing

Contributions are welcome! Open an issue or submit a pull request on GitHub.

## FAQ
- Will streaming (token-by-token) ever be supported?
    - No. This framework is designed to be more of a back-end tool, and character streaming would add extra complexity that most applications of this package would not benefit from (in my opinion).
- Are there any pre-built functions?
    - The aim of this package is to create the framework, not the functionality. If you have any ideas of useful functions, feel free to put them on an issue, and if enough arise, I can make a new repo for these.
- Where are the agents?
    - I removed the agent interface recently as I think it was far too restrictuve. I would like to instead get the core building blocks ironed out before moving on to coming up with an agent interface.

## Author

Developed by Josh Pattman. Learn more at [GitHub](https://github.com/JoshPattman/jpf).