package encoders

import (
	"github.com/JoshPattman/jpf"
)

// NewFixed creates an [Encoder] that encodes a static system prompt and raw user input as messages.
func NewFixed(systemPrompt string) jpf.Encoder[string] {
	return &fixedEncoder{
		systemPrompt: systemPrompt,
	}
}

type fixedEncoder struct {
	systemPrompt string
}

func (e *fixedEncoder) BuildInputMessages(input string) ([]jpf.Message, error) {
	messages := []jpf.Message{
		jpf.SystemMessage{
			Content: e.systemPrompt,
		},
		jpf.UserMessage{
			Content: input,
		},
	}
	return messages, nil
}
