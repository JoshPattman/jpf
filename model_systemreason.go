package jpf

// This creates a new model that switches all Reasoning roles to System roles when sending content to the model, and adds a prefix to the reasoning
func NewSystemReasonModel(model Model, prefix string) Model {
	return &systemReasonModel{
		model:  model,
		prefix: prefix,
	}
}

// This creates a new model that switches all Reasoning roles to System roles when sending content to the model, and adds a prefix to the reasoning.
// It uses a default reasoning prefix.
func NewDefaultSystemReasonModel(model Model) Model {
	return NewSystemReasonModel(
		model,
		"The following information outlines some reasoning about the conversation up to this point:\n\n",
	)
}

type systemReasonModel struct {
	model  Model
	prefix string
}

// Respond implements Model.
func (s *systemReasonModel) Respond(messages []Message) ([]Message, Message, Usage, error) {
	convertedMessages := make([]Message, len(messages))
	for i, m := range messages {
		if m.Role == ReasoningRole {
			m.Role = SystemRole
			m.Content = s.prefix + m.Content
		}
		convertedMessages[i] = m
	}
	return s.model.Respond(convertedMessages)
}

// Tokens implements Model.
func (s *systemReasonModel) Tokens() (int, int) {
	return s.model.Tokens()
}
