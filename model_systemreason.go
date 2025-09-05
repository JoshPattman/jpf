package jpf

// NewSystemReasonModel converts ReasoningRole messages to SystemRole messages.
// This allows using models that don't natively support a reasoning role by converting
// reasoning messages into system messages with a customizable prefix.
// Options:
// - WithReasoningPrefix: customizes the prefix text added before reasoning content (default provided)
func NewSystemReasonModel(model Model, opts ...systemReasonOpt) Model {
	m := &systemReasonModel{model: model, prefix: "The following information outlines some reasoning about the conversation up to this point:\n\n"}
	for _, o := range opts {
		o.applySystemReason(m)
	}
	return m
}

type systemReasonOpt interface {
	applySystemReason(*systemReasonModel)
}

func (p WithReasoningPrefix) applySystemReason(m *systemReasonModel) { m.prefix = p.X }

type systemReasonModel struct {
	model  Model
	prefix string
}

// Respond implements Model.
func (s *systemReasonModel) Respond(messages []Message) (ModelResult, error) {
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
