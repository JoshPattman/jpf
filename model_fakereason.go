package jpf

/*
// NewFakeReasoningModel creates a model that uses two underlying models to simulate reasoning.
// It first calls the reasoner model to generate reasoning about the input messages,
// then passes that reasoning along with the original messages to the answerer model.
// The reasoning is included as a ReasoningRole message in the auxiliary messages output.
// Optional parameters allow customization of the reasoning prompt.
func NewFakeReasoningModel(reasoner Model, answerer Model, opts ...fakeReasoningModelOpt) Model {
	m := &fakeReasoningModel{
		reasoner:        reasoner,
		answerer:        answerer,
		reasoningPrompt: defaultFakeReasoningPromptA,
	}
	for _, o := range opts {
		o.applyFakeReasoning(m)
	}
	return m
}

type fakeReasoningModelOpt interface {
	applyFakeReasoning(*fakeReasoningModel)
}

func (o WithReasoningPrompt) applyFakeReasoning(m *fakeReasoningModel) { m.reasoningPrompt = o.X }

const defaultFakeReasoningPromptA = `
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
func (f *fakeReasoningModel) Respond(msgs []Message) (ChatResult, error) {
	res, err := f.reasoner.Respond(append([]Message{{Role: SystemRole, Content: f.reasoningPrompt}}, msgs...))
	if err != nil {
		return res.OnlyUsage(), err
	}
	reasoning := res.Primary
	reasoning.Role = ReasoningRole
	msgsWithReasoning := append(msgs, reasoning)
	res2, err := f.answerer.Respond(msgsWithReasoning)
	usage := res.Usage.Add(res2.Usage)
	if err != nil {
		return ChatResult{Usage: usage}, err
	}
	allAux := append([]Message{reasoning}, res.Extra...)
	return ChatResult{Extra: allAux, Primary: res2.Primary, Usage: usage}, err
}

// Tokens implements Model.
func (f *fakeReasoningModel) Tokens() (int, int) {
	ai, ao := f.reasoner.Tokens()
	bi, bo := f.answerer.Tokens()
	return min(ai, bi), min(ao, bo)
}
*/
