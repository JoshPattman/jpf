package jpf

// NewRawStringMessageEncoder creates a MessageEncoder that encodes a system prompt and user input as raw string messages.
func NewRawStringMessageEncoder(systemPrompt string) MessageEncoder[string] {
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
