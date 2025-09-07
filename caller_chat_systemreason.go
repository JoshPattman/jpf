package jpf

// NewSystemReasonChatCaller converts ReasoningRole messages to SystemRole messages.
// This allows using models that don't natively support a reasoning role by converting
// reasoning messages into system messages with a customizable prefix.
// Options:
// - WithReasoningPrefix: customizes the prefix text added before reasoning content (default provided)
func NewSystemReasonChatCaller(model ChatCaller, opts ...systemReasonOpt) ChatCaller {
	m := &systemReasonChatCaller{caller: model, prefix: "The following information outlines some reasoning about the conversation up to this point:\n\n"}
	for _, o := range opts {
		o.applySystemReason(m)
	}
	return m
}

type systemReasonOpt interface {
	applySystemReason(*systemReasonChatCaller)
}

func (p WithReasoningPrefix) applySystemReason(m *systemReasonChatCaller) { m.prefix = p.X }

type systemReasonChatCaller struct {
	caller ChatCaller
	prefix string
}

// Respond implements Model.
func (s *systemReasonChatCaller) Call(messages []Message) (ChatResult, error) {
	convertedMessages := make([]Message, len(messages))
	for i, m := range messages {
		if m.Role == ReasoningRole {
			m.Role = SystemRole
			m.Content = s.prefix + m.Content
		}
		convertedMessages[i] = m
	}
	return s.caller.Call(convertedMessages)
}
