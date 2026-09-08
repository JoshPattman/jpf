package agents

// HeadStatePlacement controls where the head state (the skills section and the
// prompt fragments) is positioned in the messages sent to the model.
type HeadStatePlacement uint8

const (
	// EmbedAfterSystem appends the head state to the end of the system prompt,
	// as part of the same SystemMessage.
	EmbedAfterSystem HeadStatePlacement = iota
	// DeveloperBeforeConversation places the head state in its own
	// DeveloperMessage directly after the system prompt and before the
	// conversation. This is the default.
	DeveloperBeforeConversation
	// DeveloperAfterConversation places the head state in its own
	// DeveloperMessage after the conversation, as the final message.
	DeveloperAfterConversation
)

// ReActOpt configures a reactAgent at construction time.
type ReActOpt func(*reactAgent)

// WithHeadStatePlacement sets where the head state is placed relative to the
// system prompt and the conversation. Defaults to DeveloperBeforeConversation.
func WithHeadStatePlacement(placement HeadStatePlacement) ReActOpt {
	return func(a *reactAgent) {
		a.headStatePlacement = placement
	}
}
