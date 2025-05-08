package jpf

type Role uint8

const (
	SystemRole Role = iota
	UserRole
	AssistantRole
	ReasoningRole
)

type Message struct {
	Role    Role
	Content string
}
