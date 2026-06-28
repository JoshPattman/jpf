package jpf

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
)

// Usage defines how many tokens were used when making calls to LLMs.
type Usage struct {
	InputTokens     int
	OutputTokens    int
	SuccessfulCalls int
	FailedCalls     int
}

func (u Usage) Add(u2 Usage) Usage {
	return Usage{
		u.InputTokens + u2.InputTokens,
		u.OutputTokens + u2.OutputTokens,
		u.SuccessfulCalls + u2.SuccessfulCalls,
		u.FailedCalls + u2.FailedCalls,
	}
}

type ModelResponse struct {
	// The response to the input messages.
	Message AssistantMessage
	// The usage of making this call.
	// This may be the sum of multiple LLM calls.
	Usage Usage
}

// Utility to allow you to return the usage but 0 value messages when an error occurs.
func (r ModelResponse) OnlyUsage() ModelResponse {
	return ModelResponse{Usage: r.Usage}
}

// Utility to include another usage object in this response object
func (r ModelResponse) IncludingUsage(u Usage) ModelResponse {
	return ModelResponse{
		Message: r.Message,
		Usage:   r.Usage.Add(u),
	}
}

// Model defines an interface to an LLM.
type Model interface {
	// Responds to a set of input messages.
	Respond(context.Context, []Message, ...ModelResponseOpt) (ModelResponse, error)
}

type ModelResponseKwargs struct {
	Streamer     ModelStreamer
	OutputFormat any
	ToolSchemas  []ToolSchema
}

type ModelResponseOpt func(*ModelResponseKwargs)

// Stream the model's response to the streamer.
// This will override any previously set streamers.
func WithStreamResponse(streamer ModelStreamer) ModelResponseOpt {
	return func(mrk *ModelResponseKwargs) {
		mrk.Streamer = streamer
	}
}

// Set the output format (structured output).
// This should be a struct, as it will be processed into the correct API model output format by the model itself.
func WithOutputFormat(format any) ModelResponseOpt {
	return func(mrk *ModelResponseKwargs) {
		mrk.OutputFormat = format
	}
}

// Add the provided tool schemas (additive, not replace) to the model.
// If there is at least one tool schema, tool calling will be enabled, so the model needs to support tool calling in this case.
func WithToolSchemas(schemas ...ToolSchema) ModelResponseOpt {
	return func(mrk *ModelResponseKwargs) {
		mrk.ToolSchemas = append(mrk.ToolSchemas, schemas...)
	}
}

func GetModelResponseKwargs(opts ...ModelResponseOpt) ModelResponseKwargs {
	kw := &ModelResponseKwargs{}
	for _, o := range opts {
		o(kw)
	}
	return *kw
}

type ModelStreamer interface {
	OnMessageBegin()
	OnMessageText(text string)
	OnMessageReset()
}

// Message is a sum type of the different messages that can be sent to Models.
type Message interface {
	msg()
	String() string
	Eq(Message) bool
}

func (UserMessage) msg()      {}
func (AssistantMessage) msg() {}
func (DeveloperMessage) msg() {}
func (SystemMessage) msg()    {}

// UserMessage represents a message from the user to the model.
type UserMessage struct {
	Content string
	Images  []ImageAttachment
}

func (m UserMessage) String() string {
	return fmt.Sprintf("UserMessage{Content: \"%s\", Images: %d}", m.Content, len(m.Images))
}

func (m UserMessage) Eq(other Message) bool {
	switch other := other.(type) {
	case UserMessage:
		if len(m.Images) != len(other.Images) {
			return false
		}
		for i := range m.Images {
			if m.Images[i] != other.Images[i] {
				return false
			}
		}
		return m.Content == other.Content
	default:
		return false
	}
}

// AssistantMessage represents a message from the model to the user.
type AssistantMessage struct {
	Content string
}

func (m AssistantMessage) String() string {
	return fmt.Sprintf("AssistantMessage{Content: \"%s\"}", m.Content)
}

func (m AssistantMessage) Eq(other Message) bool {
	switch other := other.(type) {
	case AssistantMessage:
		return m.Content == other.Content
	default:
		return false
	}
}

// DeveloperMessage represents a message from the developer (basically system) to the model.
type DeveloperMessage struct {
	Content string
}

func (m DeveloperMessage) String() string {
	return fmt.Sprintf("DeveloperMessage{Content: \"%s\"}", m.Content)
}

func (m DeveloperMessage) Eq(other Message) bool {
	switch other := other.(type) {
	case DeveloperMessage:
		return m.Content == other.Content
	default:
		return false
	}
}

// SystemMessage represents a message from the system to the model, to set up its task, personality.
type SystemMessage struct {
	Content string
}

func (m SystemMessage) String() string {
	return fmt.Sprintf("SystemMessage{Content: \"%s\"}", m.Content)
}

func (m SystemMessage) Eq(other Message) bool {
	switch other := other.(type) {
	case SystemMessage:
		return m.Content == other.Content
	default:
		return false
	}
}

// ImageAttachment is an image that is attached as additional information to a message.
type ImageAttachment struct {
	Source image.Image
}

func (i *ImageAttachment) ToBase64Encoded(useCompression bool) (string, error) {
	var buf bytes.Buffer
	if useCompression {
		if err := jpeg.Encode(&buf, i.Source, &jpeg.Options{Quality: 85}); err != nil {
			return "", err
		}
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	} else {
		if err := png.Encode(&buf, i.Source); err != nil {
			return "", err
		}
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
	}
}

type ToolSchema struct {
	Name        string
	Description string
	Args        []ToolArg
}

type ToolArgType uint8

const (
	ToolArgInt ToolArgType = iota
	ToolArgFloat
	ToolArgString
)

type ToolArg struct {
	Name        string
	Description string
	Type        ToolArgType
	Required    bool
}
