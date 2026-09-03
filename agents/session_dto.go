package agents

import (
	"fmt"
	"maps"
	"slices"

	"github.com/JoshPattman/jpf"
)

// AgentSessionDTO is a JSON-serialisable representation of an AgentSession.
//
// Populate it from a session with LoadSession, and convert it back with ToSession.
//
// Round-tripping is not exactly lossless:
//
//   - Every message in CoreMessages carries the jpf.MessageDTO caveats (tool-arg
//     numbers come back as float64, image attachments are re-encoded as PNG and
//     will not compare Eq).
//   - The arguments of a deferred tool call also pass through JSON as float64. These
//     args have already been validated and coerced against their tool schema (e.g.
//     ints stored as int), and that coercion is undone by the round-trip.
//     However, it is expected the user of the framework will have triggered
//     the background job already for that call, so this should not matter.
type AgentSessionDTO struct {
	AgentPrompt              string                `json:"agent_prompt"`
	TaskPrompt               string                `json:"task_prompt"`
	PersonalityPrompt        string                `json:"personality_prompt"`
	CoreMessages             []jpf.MessageDTO      `json:"core_messages,omitempty"`
	CurrentDeferredToolCalls []DeferredToolCallDTO `json:"current_deferred_tool_calls,omitempty"`
	ActiveSkillNames         []string              `json:"active_skill_names,omitempty"`
}

// DeferredToolCallDTO is a JSON-serialisable representation of a DeferredToolCall.
type DeferredToolCallDTO struct {
	ToolName string         `json:"tool_name"`
	CallID   string         `json:"call_id"`
	Args     map[string]any `json:"args,omitempty"`
}

// LoadDeferredToolCall populates the DTO in place from call, replacing any existing contents.
func (d *DeferredToolCallDTO) LoadDeferredToolCall(call DeferredToolCall) {
	*d = DeferredToolCallDTO{
		ToolName: call.ToolName,
		CallID:   call.CallID,
		Args:     maps.Clone(call.Args),
	}
}

// ToDeferredToolCall converts the DTO back into the DeferredToolCall it represents.
func (d *DeferredToolCallDTO) ToDeferredToolCall() DeferredToolCall {
	return DeferredToolCall{
		ToolName: d.ToolName,
		CallID:   d.CallID,
		Args:     maps.Clone(d.Args),
	}
}

// LoadSession populates the DTO in place from sess, replacing any existing contents.
func (d *AgentSessionDTO) LoadSession(sess AgentSession) error {
	*d = AgentSessionDTO{
		AgentPrompt:       sess.AgentPrompt,
		TaskPrompt:        sess.TaskPrompt,
		PersonalityPrompt: sess.PersonalityPrompt,
		ActiveSkillNames:  slices.Clone(sess.ActiveSkillNames),
	}
	for i, msg := range sess.CoreMessages {
		var msgDTO jpf.MessageDTO
		if err := msgDTO.LoadMessage(msg); err != nil {
			return fmt.Errorf("failed to load core message %d: %w", i, err)
		}
		d.CoreMessages = append(d.CoreMessages, msgDTO)
	}
	for _, call := range sess.CurrentDeferredToolCalls {
		var callDTO DeferredToolCallDTO
		callDTO.LoadDeferredToolCall(call)
		d.CurrentDeferredToolCalls = append(d.CurrentDeferredToolCalls, callDTO)
	}
	return nil
}

// ToSession converts the DTO back into the AgentSession it represents.
func (d *AgentSessionDTO) ToSession() (AgentSession, error) {
	sess := AgentSession{
		AgentPrompt:       d.AgentPrompt,
		TaskPrompt:        d.TaskPrompt,
		PersonalityPrompt: d.PersonalityPrompt,
		ActiveSkillNames:  slices.Clone(d.ActiveSkillNames),
	}
	for i, msgDTO := range d.CoreMessages {
		msg, err := msgDTO.ToMessage()
		if err != nil {
			return AgentSession{}, fmt.Errorf("failed to convert core message %d: %w", i, err)
		}
		sess.CoreMessages = append(sess.CoreMessages, msg)
	}
	for _, callDTO := range d.CurrentDeferredToolCalls {
		sess.CurrentDeferredToolCalls = append(sess.CurrentDeferredToolCalls, callDTO.ToDeferredToolCall())
	}
	return sess, nil
}
