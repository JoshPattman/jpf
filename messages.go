package jpf

type Role uint8

const (
	SystemRole Role = iota
	UserRole
	AssistantRole
)

type Message struct {
	Role    Role
	Content string
}
