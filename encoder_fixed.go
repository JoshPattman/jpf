package jpf

// NewFixedEncoder creates an [Encoder] that encodes a static system prompt and raw user input as messages.
func NewFixedEncoder(systemPrompt string) Encoder[string] {
	return &fixedEncoder{
		systemPrompt: systemPrompt,
	}
}

type fixedEncoder struct {
	systemPrompt string
}

func (e *fixedEncoder) BuildInputMessages(input string) ([]Message, error) {
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
