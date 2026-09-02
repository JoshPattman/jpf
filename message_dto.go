package jpf

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"slices"
	"strings"
)

// MessageRole identifies which concrete Message type a MessageDTO represents.
type MessageRole string

const (
	MessageRoleUser       MessageRole = "user"
	MessageRoleAssistant  MessageRole = "assistant"
	MessageRoleDeveloper  MessageRole = "developer"
	MessageRoleSystem     MessageRole = "system"
	MessageRoleToolResult MessageRole = "tool_result"
)

// MessageDTO is a JSON-serialisable representation of a Message.
//
// Populate it from a Message with LoadMessage, and convert it back with ToMessage.
// Only the fields relevant to the Role are set; the rest stay at their zero value
// and are omitted from JSON.
//
// Round-tripping is not exactly lossless in two ways:
//
//   - Tool call arguments pass through JSON, so numeric values come back as float64
//     regardless of how they went in - the same normalisation the agent framework
//     already applies to arguments returned by a model.
//   - Image attachments on a UserMessage are re-encoded as base64 PNG data URIs by
//     LoadMessage and decoded into fresh image.Image values by ToMessage. The pixels
//     survive, but any non-PNG source encoding does not, and because the decoded
//     image is a new value the resulting UserMessage will not compare Eq to the
//     original.
type MessageDTO struct {
	Role MessageRole `json:"role"`
	// Text content, for user, assistant, developer and system messages.
	Content string `json:"content,omitempty"`
	// Images attached to a user message, each as a "data:image/png;base64,..." URI.
	// ToMessage decodes these back into image.Image values; see the type doc for the
	// round-trip caveats.
	Images []string `json:"images,omitempty"`
	// Requested tool calls, for assistant messages.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// The id of the tool call being responded to, for tool result messages.
	CallID string `json:"call_id,omitempty"`
	// The tool output, for tool result messages.
	Result string `json:"result,omitempty"`
}

// LoadMessage populates the DTO in place from msg, replacing any existing contents.
func (d *MessageDTO) LoadMessage(msg Message) error {
	*d = MessageDTO{}
	switch msg := msg.(type) {
	case UserMessage:
		d.Role = MessageRoleUser
		d.Content = msg.Content
		for _, img := range msg.Images {
			encoded, err := img.ToBase64Encoded(false)
			if err != nil {
				return fmt.Errorf("failed to encode image attachment: %w", err)
			}
			d.Images = append(d.Images, encoded)
		}
	case AssistantMessage:
		d.Role = MessageRoleAssistant
		d.Content = msg.Content
		d.ToolCalls = slices.Clone(msg.ToolCalls)
	case DeveloperMessage:
		d.Role = MessageRoleDeveloper
		d.Content = msg.Content
	case SystemMessage:
		d.Role = MessageRoleSystem
		d.Content = msg.Content
	case ToolResultMessage:
		d.Role = MessageRoleToolResult
		d.CallID = msg.CallID
		d.Result = msg.Result
	default:
		return fmt.Errorf("cannot load message of unknown type %T", msg)
	}
	return nil
}

// ToMessage converts the DTO into the concrete Message that its Role describes.
func (d *MessageDTO) ToMessage() (Message, error) {
	switch d.Role {
	case MessageRoleUser:
		var images []ImageAttachment
		for _, encoded := range d.Images {
			img, err := decodeDataURIImage(encoded)
			if err != nil {
				return nil, fmt.Errorf("failed to decode image attachment: %w", err)
			}
			images = append(images, ImageAttachment{Source: img})
		}
		return UserMessage{Content: d.Content, Images: images}, nil
	case MessageRoleAssistant:
		return AssistantMessage{Content: d.Content, ToolCalls: slices.Clone(d.ToolCalls)}, nil
	case MessageRoleDeveloper:
		return DeveloperMessage{Content: d.Content}, nil
	case MessageRoleSystem:
		return SystemMessage{Content: d.Content}, nil
	case MessageRoleToolResult:
		return ToolResultMessage{CallID: d.CallID, Result: d.Result}, nil
	default:
		return nil, fmt.Errorf("cannot convert message with unknown role %q", d.Role)
	}
}

// decodeDataURIImage decodes an image from a "data:<mime>;base64,<data>" URI,
// tolerating a bare base64 string with no data-URI prefix.
func decodeDataURIImage(encoded string) (image.Image, error) {
	data := encoded
	if after, ok := strings.CutPrefix(data, "data:"); ok {
		_, b64, found := strings.Cut(after, ",")
		if !found {
			return nil, fmt.Errorf("malformed data URI: missing ','")
		}
		data = b64
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return img, nil
}
