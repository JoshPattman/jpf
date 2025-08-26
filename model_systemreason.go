package jpf

import "fmt"

type SystemReasonModelBuilder struct {
	builder ModelBuilder
	prefix  string
}

func BuildSystemReasonModel(builder ModelBuilder) *SystemReasonModelBuilder {
	return &SystemReasonModelBuilder{
		builder: builder,
		prefix:  "The following information outlines some reasoning about the conversation up to this point:\n\n",
	}
}

func (b *SystemReasonModelBuilder) New() (Model, error) {
	if b.builder == nil {
		return nil, fmt.Errorf("cannot have a nil builder")
	}
	subModel, err := b.builder.New()
	if err != nil {
		return nil, err
	}
	return &systemReasonModel{
		model:  subModel,
		prefix: b.prefix,
	}, nil
}

func (b *SystemReasonModelBuilder) WithPrefix(prefix string) *SystemReasonModelBuilder {
	b.prefix = prefix
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
