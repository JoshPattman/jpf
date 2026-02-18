package jpf

import (
	"bytes"
	"context"
	"encoding/base64"
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
	Message Message
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
	Respond(context.Context, []Message) (ModelResponse, error)
}

// Role is an enum specifying a role for a message.
// It is not 1:1 with openai roles (i.e. there is a reasoning role here).
type Role uint8

const (
	SystemRole Role = iota
	UserRole
	AssistantRole
	ReasoningRole
	DeveloperRole
)

func (r Role) String() string {
	switch r {
	case SystemRole:
		return "system"
	case UserRole:
		return "user"
	case AssistantRole:
		return "assistant"
	case ReasoningRole:
		return "reasoning"
	case DeveloperRole:
		return "developer"
	}
	panic("not a valid role")
}

// Message defines a text message to/from an LLM.
type Message struct {
	Role    Role
	Content string
	Images  []ImageAttachment
}

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
