package jpf

type Usage struct {
	InputTokens  int
	OutputTokens int
}

type Model interface {
	Tokens() (int, int)
	Respond([]Message) (Message, Usage, error)
}
