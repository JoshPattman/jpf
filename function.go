package jpf

import (
	"errors"
)

var (
	ErrInvalidResponse = errors.New("llm produced an invalid response")
)

// A MessageEncoder encodes a structured pice of data into a set of messages for an LLM.
type MessageEncoder[T any] interface {
	BuildInputMessages(T) ([]Message, error)
}

// a ResponseDecoder converts an LLM response into a structured peice of data.
// When the LLM responsee was invalid, it should return ErrInvalidResponse (or an error joined on that).
type ResponseDecoder[T any] interface {
	ParseResponseText(string) (T, error)
}

// A FeedbackGenerator can take an error and convert it to a pice of text feedback to send to the LLM.
type FeedbackGenerator interface {
	FormatFeedback(Message, error) string
}

type MapFunc[T, U any] interface {
	Call(T) (U, Usage, error)
}

func NewOneShotMapFunc[T, U any](enc MessageEncoder[T], pars ResponseDecoder[U], model Model) MapFunc[T, U] {
	return &oneShotMapFunc[T, U]{
		enc:   enc,
		pars:  pars,
		model: model,
	}
}

type oneShotMapFunc[T, U any] struct {
	enc   MessageEncoder[T]
	pars  ResponseDecoder[U]
	model Model
}

func (mf *oneShotMapFunc[T, U]) Call(t T) (U, Usage, error) {
	var u U
	msgs, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return u, Usage{}, err
	}
	_, resp, usage, err := mf.model.Respond(msgs)
	if err != nil {
		return u, usage, err
	}
	result, err := mf.pars.ParseResponseText(resp.Content)
	if err != nil {
		return u, usage, err
	}
	return result, usage, nil
}

// Creates a map func that will keep adding to the conversation with feedback when errors are detected.
// It will only ever add to the conversation if the error returned from fed is a ErrInvalidResponse (using errors.Is)
func NewFeedbackMapFunc[T, U any](
	enc MessageEncoder[T],
	pars ResponseDecoder[U],
	fed FeedbackGenerator,
	model Model,
	feedbackRole Role,
	maxRetries int,
) MapFunc[T, U] {
	return &feedbackMapFunc[T, U]{
		enc:          enc,
		pars:         pars,
		fed:          fed,
		model:        model,
		feedbackRole: feedbackRole,
		maxRetries:   maxRetries,
	}
}

type feedbackMapFunc[T, U any] struct {
	enc          MessageEncoder[T]
	pars         ResponseDecoder[U]
	fed          FeedbackGenerator
	model        Model
	feedbackRole Role
	maxRetries   int
}

func (mf *feedbackMapFunc[T, U]) Call(t T) (U, Usage, error) {
	var u U
	history, err := mf.enc.BuildInputMessages(t)
	if err != nil {
		return u, Usage{}, err
	}
	totalUsage := Usage{}
	var lastErr error
	for range mf.maxRetries {
		_, resp, usage, err := mf.model.Respond(history)
		totalUsage = totalUsage.Add(usage)
		if err != nil {
			return u, totalUsage, err
		}
		result, err := mf.pars.ParseResponseText(resp.Content)
		if err == nil {
			// If the result was ok, return it
			return result, totalUsage, nil
		} else if errors.Is(err, ErrInvalidResponse) {
			// If it was a parse error, add to the conversation history and continue looping
			feedback := mf.fed.FormatFeedback(resp, err)
			lastErr = err
			history = append(history, resp)
			history = append(history, Message{
				Role:    mf.feedbackRole,
				Content: feedback,
			})
		} else {
			// Otherwise, it was another error so return the error (don't loop)
			return u, totalUsage, err
		}
	}
	return u, totalUsage, lastErr
}

// NewRawMessageFeedbackGenerator creates a FeedbackGenerator that formats feedback by returning the error message as a string.
func NewRawMessageFeedbackGenerator() FeedbackGenerator {
	return &rawMessageFeedbackGenerator{}
}

type rawMessageFeedbackGenerator struct{}

func (g *rawMessageFeedbackGenerator) FormatFeedback(_ Message, err error) string {
	return err.Error()
}

// NewRawStringMessageEncoder creates a MessageEncoder that encodes a system prompt and user input as raw string messages.
func NewRawStringMessageEncoder(systemPrompt string) *rawStringMessageEncoder {
	return &rawStringMessageEncoder{
		systemPrompt: systemPrompt,
	}
}

type rawStringMessageEncoder struct {
	systemPrompt string
}

func (e *rawStringMessageEncoder) BuildInputMessages(input string) ([]Message, error) {
	messages := []Message{
		{
			Role:    SystemRole,
			Content: e.systemPrompt,
		},
		{
			Role:    UserRole,
			Content: input,
		},
	}
	return messages, nil
}

// NewRawStringResponseDecoder creates a ResponseDecoder that returns the response as a raw string without modification.
func NewRawStringResponseDecoder() *rawStringResponseDecoder {
	return &rawStringResponseDecoder{}
}

type rawStringResponseDecoder struct{}

func (d *rawStringResponseDecoder) DecodeResponse(response string) (string, error) {
	return response, nil
}
