package jpf

import (
	"fmt"
)

// A function is a short-lived task-specific LLM configuration.
// They are intended to be used to perform single tasks, and should not be used for long-running conversations.
type Function[T, U any] interface {
	// Create the input messages from the input value
	BuildInputMessages(T) ([]Message, error)
	// Parse the raw LLM response into an output value,
	// returning a [ParseError] if the response cannot be parsed, or another error if somthing else.
	ParseResponseText(string) (U, error)
}

// A parse error is a specific type of error that is created when a function fails to parse an LLM response
type ParseError struct {
	// The latest response of the LLM
	Response string
	// The error that occured
	Err error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse: '%s'", e.Err)
}

// A retry function is a special type of function that can provide feedback to the LLM when the parse failed.
// It is still not intended for long-running conversations, however under the hood it does create a long conversation until the parse is sucsessful.
type RetryFunction[T, U any] interface {
	Function[T, U]
	// Format the feedback from this parse error
	FormatFeedback(*ParseError) string
}

// Runs a function with one try,
// i.e. it asks the LLM once and tries to parse once.
func RunOneShot[T, U any](model Model, f Function[T, U], input T) (U, Usage, error) {
	var u U
	msgs, err := f.BuildInputMessages(input)
	if err != nil {
		return u, Usage{}, err
	}
	resp, usage, err := model.Respond(msgs)
	if err != nil {
		return u, usage, err
	}
	result, err := f.ParseResponseText(resp.Content)
	if err != nil {
		return u, usage, err
	}
	return result, usage, nil
}

// Runs a function with a number of retries, providing feedback at each parse fail,
// i.e. asks the llm the inital messages at the start of the conversation and continues to provide feedback until the answer is parseable.
func RunWithRetries[T, U any](model Model, f RetryFunction[T, U], maxRetries int, input T) (U, Usage, error) {
	var u U
	history, err := f.BuildInputMessages(input)
	if err != nil {
		return u, Usage{}, err
	}
	totalUsage := Usage{}
	var lastErr *ParseError
	for range maxRetries {
		resp, usage, err := model.Respond(history)
		totalUsage.InputTokens += usage.InputTokens
		totalUsage.OutputTokens += usage.OutputTokens
		if err != nil {
			return u, totalUsage, err
		}
		result, err := f.ParseResponseText(resp.Content)
		if err == nil {
			// If the result was ok, return it
			return result, totalUsage, nil
		} else if parseErr, ok := err.(*ParseError); ok {
			// If it was a parse error, add to the conversation history and continue looping
			feedback := f.FormatFeedback(parseErr)
			lastErr = parseErr
			history = append(history, Message{
				Role:    AssistantRole,
				Content: parseErr.Response,
			})
			history = append(history, Message{
				Role:    UserRole,
				Content: feedback,
			})
		} else {
			// Otherwise, it was another error so return the error (don't loop)
			return u, totalUsage, err
		}
	}
	return u, totalUsage, lastErr
}
