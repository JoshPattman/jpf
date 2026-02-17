package encoders

import (
	"github.com/JoshPattman/jpf"
)

// NewFixedEncoder creates an [Encoder] that encodes a static system prompt and raw user input as messages.
func NewFixedEncoder(systemPrompt string) jpf.Encoder[string] {
	return &fixedEncoder{
		systemPrompt: systemPrompt,
	}
}

type fixedEncoder struct {
	systemPrompt string
}

func (e *fixedEncoder) BuildInputMessages(input string) ([]jpf.Message, error) {
	messages := []jpf.Message{
		{
			Role:    jpf.SystemRole,
			Content: e.systemPrompt,
		},
		{
			Role:    jpf.UserRole,
			Content: input,
		},
	}
	return messages, nil
}
