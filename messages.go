package jpf

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"image/png"
)

// Role is an enum specifying a role for a message.
// It is not 1:1 with openai roles (i.e. there is a reasoning role here).
type Role uint8

const (
	SystemRole Role = iota
	UserRole
	AssistantRole
	ReasoningRole
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
