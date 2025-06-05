package jpf

type SystemReasinModelBuilder struct {
	model *systemReasonModel
}

func BuildSystemReasonModel(model Model) *SystemReasinModelBuilder {
	return &SystemReasinModelBuilder{
		model: &systemReasonModel{
			model:  model,
			prefix: "The following information outlines some reasoning about the conversation up to this point:\n\n",
		},
	}
}

func (b *SystemReasinModelBuilder) Validate() (Model, error) {
	return b.model, nil
}

func (b *SystemReasinModelBuilder) WithPrefix(prefix string) *SystemReasinModelBuilder {
	b.model.prefix = prefix
	return b
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
