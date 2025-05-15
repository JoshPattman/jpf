package jpf

// Role is an enum specifying a role for a message.
// It is not 1:1 with openai roles (i.e. there is a reasoning role here).
type Role uint8

const (
	SystemRole Role = iota
	UserRole
	AssistantRole
	ReasoningRole
)

// Message defines a text message to/from an LLM.
type Message struct {
	Role    Role
	Content string
}
