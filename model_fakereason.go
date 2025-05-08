package jpf

// Creates a new model that simulates reasoning by asking one model to reason and the other to answer.
func NewFakeReasoningModel(reasoner Model, answerer Model, reasoningPrompt string) Model {
	return &fakeReasoningModel{
		reasoner:        reasoner,
		answerer:        answerer,
		reasoningPrompt: reasoningPrompt,
	}
}

const DefaultFakeReasoningPromptA = `
- You are a specialised reasoning AI, tasked with reasoning about another AIs task.
- Your job is to provide prior reasoning another AI model, to assist it in answering its question accurately.
- You should look at the messages up to then end of the conversation, along with the following system prompt (for the other model), and reason.
	- Following system prompts will be designed for the other model - this system prompt will always be valid.
- You should think step-by-step, breaking your answer down into small chunks.
`

type fakeReasoningModel struct {
	reasoner        Model
	answerer        Model
	reasoningPrompt string
}

// Respond implements Model.
func (f *fakeReasoningModel) Respond(msgs []Message) ([]Message, Message, Usage, error) {
	_, reasoning, usage, err := f.reasoner.Respond(append([]Message{{Role: SystemRole, Content: f.reasoningPrompt}}, msgs...))
	if err != nil {
		return nil, Message{}, usage, err
	}
	reasoning.Role = ReasoningRole
	msgsWithReasoning := append(msgs, reasoning)
	aux, msg, usage2, err := f.answerer.Respond(msgsWithReasoning)
	usage = usage.Add(usage2)
	allAux := append([]Message{reasoning}, aux...)
	return allAux, msg, usage, err
}

// Tokens implements Model.
func (f *fakeReasoningModel) Tokens() (int, int) {
	ai, ao := f.reasoner.Tokens()
	bi, bo := f.answerer.Tokens()
	return min(ai, bi), min(ao, bo)
}
