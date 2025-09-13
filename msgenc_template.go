package jpf

import (
	"bytes"
	"text/template"
)

// NewTemplateMessageEncoder creates a MessageEncoder that uses Go's text/template for formatting messages.
// It accepts templates for both system and user messages, allowing dynamic content insertion.
// The data parameter to BuildInputMessages should be a struct or map with fields accessible to the template.
// If either systemTemplate or userTemplate is an empty string, that message will be skipped.
func NewTemplateMessageEncoder[T any](systemTemplate, userTemplate string) MessageEncoder[T] {
	encoder := &templateMessageEncoder[T]{}

	if systemTemplate != "" {
		encoder.systemTemplate = template.Must(template.New("system").Parse(systemTemplate))
	}

	if userTemplate != "" {
		encoder.userTemplate = template.Must(template.New("user").Parse(userTemplate))
	}

	return encoder
}

type templateMessageEncoder[T any] struct {
	systemTemplate *template.Template
	userTemplate   *template.Template
}

func (e *templateMessageEncoder[T]) BuildInputMessages(data T) ([]Message, error) {
	messages := []Message{}

	if e.systemTemplate != nil {
		var systemBuf bytes.Buffer
		if err := e.systemTemplate.Execute(&systemBuf, data); err != nil {
			return nil, wrap(err, "failed to execute system prompt template")
		}
		messages = append(messages, Message{
			Role:    SystemRole,
			Content: systemBuf.String(),
		})
	}

	if e.userTemplate != nil {
		var userBuf bytes.Buffer
		if err := e.userTemplate.Execute(&userBuf, data); err != nil {
			return nil, wrap(err, "failed to execute user prompt template")
		}
		messages = append(messages, Message{
			Role:    UserRole,
			Content: userBuf.String(),
		})
	}

	return messages, nil
}
