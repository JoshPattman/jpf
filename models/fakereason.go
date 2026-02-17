package models

import (
	"context"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/utils"
)

// TwoStageReason creates a model that uses two underlying models to simulate reasoning.
// It first calls the reasoner model to generate reasoning about the input messages,
// then passes that reasoning along with the original messages to the answerer model.
// The reasoning is included as a ReasoningRole message in the auxiliary messages output.
// Optional parameters allow customization of the reasoning prompt.
func TwoStageReason(reasoner jpf.Model, answerer jpf.Model, opts ...TwoStageReasonOpt) jpf.Model {
	m := &fakeReasoningModel{
		reasoner:        reasoner,
		answerer:        answerer,
		reasoningPrompt: defaultFakeReasoningPromptA,
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

type TwoStageReasonOpt func(*fakeReasoningModel)

func WithReasoningPrompt(prompt string) func(m *fakeReasoningModel) {
	return func(m *fakeReasoningModel) { m.reasoningPrompt = prompt }
}

const defaultFakeReasoningPromptA = `
- You are a specialised reasoning AI, tasked with reasoning about another AIs task.
- Your job is to provide prior reasoning another AI model, to assist it in answering its question accurately.
- You should look at the messages up to then end of the conversation, along with the following system prompt (for the other model), and reason.
	- Following system prompts will be designed for the other model - this system prompt will always be valid.
- You should think step-by-step, breaking your answer down into small chunks.
`

type fakeReasoningModel struct {
	reasoner        jpf.Model
	answerer        jpf.Model
	reasoningPrompt string
}

// Respond implements Model.
func (f *fakeReasoningModel) Respond(ctx context.Context, msgs []jpf.Message) (jpf.ModelResponse, error) {
	reasoningResp, err := f.reasoner.Respond(ctx, append([]jpf.Message{{Role: jpf.SystemRole, Content: f.reasoningPrompt}}, msgs...))
	if err != nil {
		return reasoningResp.OnlyUsage(), utils.Wrap(err, "failed to call reasoning model")
	}
	reasoningMessage := reasoningResp.PrimaryMessage
	reasoningMessage.Role = jpf.ReasoningRole
	msgsWithReasoning := append(msgs, reasoningMessage)
	finalResp, err := f.answerer.Respond(ctx, msgsWithReasoning)
	finalResp = finalResp.IncludingUsage(reasoningResp.Usage)
	if err != nil {
		return finalResp.OnlyUsage(), utils.Wrap(err, "failed to call final response model")
	}
	finalResp.AuxiliaryMessages = append(finalResp.AuxiliaryMessages, reasoningMessage)
	return finalResp, nil
}
