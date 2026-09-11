package agents

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

func NewReAct(model jpf.Model, opts ...ReActOpt) jpf.Agent {
	a := &reactAgent{
		session:            jpf.DefaultAgentSession(),
		maxIterations:      20,
		model:              model,
		headStatePlacement: DeveloperBeforeConversation,
	}
	for _, opt := range opts {
		opt(a)
	}
	a.SetSkillCatalogue(nil)
	a.SetToolCatalogue(nil)
	return a
}

type reactAgent struct {
	session               jpf.AgentSession
	toolCatalogue         []jpf.Tool
	skillCatalogue        []jpf.Skill
	maxIterations         int
	model                 jpf.Model
	headStatePlacement    HeadStatePlacement
	headStateCallbacks    []func(jpf.AgentSession) map[string]string
	headStateCallbackMode HeadStateCallbackMode
}

func (a *reactAgent) Session() jpf.AgentSession {
	return a.session.Clone()
}

func (a *reactAgent) SetSession(sess jpf.AgentSession) {
	a.session = sess.Clone()
}

func (a *reactAgent) SetMaxIterations(n int) {
	a.maxIterations = n
}

func (a *reactAgent) SetToolCatalogue(tools []jpf.Tool) {
	a.toolCatalogue = slices.Concat(a.getBuiltinTools(), slices.Clone(tools))
}

func (a *reactAgent) SetSkillCatalogue(skills []jpf.Skill) {
	a.skillCatalogue = slices.Clone(skills)
}

func (a *reactAgent) SetHeadStateCallbacks(cbs []func(jpf.AgentSession) map[string]string) {
	a.headStateCallbacks = slices.Clone(cbs)
}

// Run the agent from a new message to add into the conversation.
// Should only be called if the agent is not currently awaiting deferred tool responses.
// May terminate because the agent is done, has hit max iterations, or is awaiting deferred tool responses.
func (a *reactAgent) Run(ctx context.Context, query string, opts ...jpf.AgentResponseOpt) error {
	kwargs := jpf.GetAgentResponseKwargs(opts)
	if len(a.Session().CurrentDeferredToolCalls) != 0 {
		return fmt.Errorf("cannot run an agent from fresh when it is awaiting deferred calls, please use resume instead")
	}
	msg := jpf.UserMessage{Content: query}
	a.session.CoreMessages = append(a.session.CoreMessages, msg)
	kwargs.Streamer.OnMessageComplete(msg)
	if a.headStateCallbackMode == BeforeEachTurn {
		a.applyFragmentsFromCallbacks()
	}
	return a.runOrResumeHelper(ctx, kwargs)
}

// Resume the agent from a set of responses to deferred tool calls to add into the conversation.
// Should only be called if the agent is currently awaiting deferred tool responses.
// May terminate because the agent is done, has hit max iterations, or is awaiting deferred tool responses.
func (a *reactAgent) Resume(ctx context.Context, callResults []jpf.DeferredCallResult, opts ...jpf.AgentResponseOpt) error {
	kwargs := jpf.GetAgentResponseKwargs(opts)
	defCalls := a.Session().CurrentDeferredToolCalls
	if len(defCalls) == 0 {
		return fmt.Errorf("cannot resume an agent when it is not awaiting deferred calls, please use run instead")
	}
	if len(defCalls) != len(callResults) {
		return fmt.Errorf("call results do not match the expected awaiting deferred calls")
	}
	// Verify required calls are present
	callIDs := make([]string, len(defCalls))
	for i, call := range defCalls {
		callIDs[i] = call.CallID
	}
	for _, res := range callResults {
		if !slices.Contains(callIDs, res.CallID) {
			return fmt.Errorf("call results do not match the expected awaiting deferred calls")
		}
	}
	// Replace placeholder calls
	for _, result := range callResults {
		for i := len(a.session.CoreMessages) - 1; i >= 0; i-- {
			resp, ok := a.session.CoreMessages[i].(jpf.ToolResultMessage)
			if !ok {
				continue
			}
			if resp.CallID != result.CallID {
				continue
			}
			if result.Err != nil {
				resp.Result = fmt.Sprintf("The tool call failed with error: %s", result.Err.Error())
			} else {
				resp.Result = result.Content
				// See the note in executeToolCalls: fragments from deferred calls
				// are applied after those from direct calls, not in tool-call order.
				a.applyFragments(result.PromptFragments)
			}
			a.session.CoreMessages[i] = resp
			break
		}
	}
	// Run callback
	for i, msg := range slices.Backward(a.session.CoreMessages) {
		_, ok := msg.(jpf.ToolResultMessage)
		if !ok {
			for j := i + 1; j < len(a.session.CoreMessages); j++ {
				kwargs.Streamer.OnMessageComplete(a.session.CoreMessages[j])
			}
			break
		}
	}

	a.session.CurrentDeferredToolCalls = nil
	return a.runOrResumeHelper(ctx, kwargs)
}

func (a *reactAgent) runOrResumeHelper(ctx context.Context, kwargs jpf.AgentResponseKwargs) error {
	a.deactivateMissingActiveSkills()
	for range a.maxIterations {
		if a.headStateCallbackMode == BeforeEachToolIteration {
			a.applyFragmentsFromCallbacks()
		}
		nextAction, err := a.determineNextAction(ctx, kwargs.Streamer.OnMessageComplete)
		if err != nil {
			return utils.Wrap(err, "failed to determine next action")
		}
		if len(nextAction.ToolCalls) == 0 {
			break
		} else {
			err = a.executeToolCalls(ctx, kwargs.Streamer.OnMessageComplete, nextAction)
			if err != nil {
				return utils.Wrap(err, "failed to execute tools")
			}
		}
		if len(a.session.CurrentDeferredToolCalls) > 0 {
			break
		}
	}
	return nil
}

func (a *reactAgent) deactivateMissingActiveSkills() {
	nextActiveSkills := make([]string, 0)
	for _, s := range a.session.ActiveSkillNames {
		_, err := a.lookupSkill(s)
		if err == nil {
			nextActiveSkills = append(nextActiveSkills, s)
		}
	}
	a.session.ActiveSkillNames = nextActiveSkills
}

func (a *reactAgent) getBuiltinTools() []jpf.Tool {
	activateSkillTool := jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "activate_skill",
			Description: "activate a skill that is currently not active, causing the full skill body to show in all future context for you (in the head state)",
			Params: []jpf.ToolParam{
				{
					Name:        "skill_name",
					Description: "the name of the skill to activate",
					Type:        jpf.ToolParamString,
				},
			},
		},
		Call: func(_ context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
			name := m.RequiredString("skill_name")
			if slices.Contains(a.session.ActiveSkillNames, name) {
				return jpf.ToolResult{}, fmt.Errorf("skill '%s' is already active", name)
			}
			skill, err := a.lookupSkill(name)
			if err != nil {
				return jpf.ToolResult{}, err
			}
			a.session.ActiveSkillNames = append(a.session.ActiveSkillNames, skill.Name)
			return jpf.ToolResult{Content: fmt.Sprintf("activated skill '%s'", skill.Name)}, nil
		},
	}

	deactivateSkillTool := jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "deactivate_skill",
			Description: "deactivate a skill that is currently active, causing the full skill body to be removed in future calls (in the head state) - call this when you feel a skill is no longer useful to you and you are able to forget it for now",
			Params: []jpf.ToolParam{
				{
					Name:        "skill_name",
					Description: "the name of the skill to deactivate",
					Type:        jpf.ToolParamString,
				},
			},
		},
		Call: func(_ context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
			name := m.RequiredString("skill_name")
			if !slices.Contains(a.session.ActiveSkillNames, name) {
				return jpf.ToolResult{}, fmt.Errorf("skill '%s' is not currently active", name)
			}
			skill, err := a.lookupSkill(name)
			if err != nil {
				return jpf.ToolResult{}, err
			}
			a.session.ActiveSkillNames = slices.DeleteFunc(a.session.ActiveSkillNames, func(s string) bool { return s == skill.Name })
			return jpf.ToolResult{Content: fmt.Sprintf("deactivated skill '%s'", skill.Name)}, nil
		},
	}
	return []jpf.Tool{
		activateSkillTool,
		deactivateSkillTool,
	}
}

func (a *reactAgent) determineNextAction(ctx context.Context, messageCallback func(jpf.Message)) (jpf.AssistantMessage, error) {
	llmMessages := a.getMessagesForLLM()
	response, err := a.model.Respond(
		ctx,
		llmMessages,
		jpf.WithToolSchemas(a.toolSchemas()...),
	)
	if err != nil {
		return jpf.AssistantMessage{}, err
	}
	a.session.CoreMessages = append(a.session.CoreMessages, response.Message)
	messageCallback(response.Message)
	return response.Message, nil
}

func (a *reactAgent) executeToolCalls(ctx context.Context, messageCallback func(jpf.Message), action jpf.AssistantMessage) error {
	tools := make([]jpf.Tool, len(action.ToolCalls))
	for i, call := range action.ToolCalls {
		tool, err := a.lookupTool(call.Tool)
		if err != nil {
			tools[i] = jpf.Tool{
				Call: func(ctx context.Context, m jpf.ToolArgs) (jpf.ToolResult, error) {
					return jpf.ToolResult{}, fmt.Errorf("could not find tool with name '%s'", call.Tool)
				},
			}
		} else {
			tools[i] = tool
		}
	}

	deferredCalls := make([]jpf.DeferredToolCall, 0)
	toCallback := make([]jpf.ToolResultMessage, 0)

	for i, call := range action.ToolCalls {
		msg := jpf.ToolResultMessage{CallID: call.ID}

		args := call.Args.Clone()
		err := args.AlignWithSchema(tools[i].Schema)
		if err != nil {
			msg.Result = fmt.Sprintf("The tool call failed with error: %s", err.Error())
		} else if tools[i].Call == nil {
			msg.Result = ""
			deferredCalls = append(deferredCalls, jpf.DeferredToolCall{ToolName: call.Tool, CallID: msg.CallID, Args: args})
		} else {
			result, err := tools[i].Call(ctx, args)
			if err != nil {
				msg.Result = fmt.Sprintf("The tool call failed with error: %s", err.Error())
			} else {
				msg.Result = result.Content
				// NOTE: Fragments are not applied in tool-call order. Fragments
				// from directly-run tools are applied first, then those from
				// deferred calls when they resume. This is expected to be a rare
				// edge case, so the extra code needed to fix it is not worth it.
				a.applyFragments(result.PromptFragments)
			}
		}
		a.session.CoreMessages = append(a.session.CoreMessages, msg)
		toCallback = append(toCallback, msg)
	}

	if len(deferredCalls) == 0 {
		for _, msg := range toCallback {
			messageCallback(msg)
		}
	} else {
		a.session.CurrentDeferredToolCalls = append(a.session.CurrentDeferredToolCalls, deferredCalls...)
	}

	return nil
}

func (a *reactAgent) lookupTool(name string) (jpf.Tool, error) {
	for _, t := range a.toolCatalogue {
		if t.Schema.Name == name {
			return t, nil
		}
	}
	return jpf.Tool{}, fmt.Errorf("could not find tool with name '%s'", name)
}

func (a *reactAgent) toolSchemas() []jpf.ToolSchema {
	schemas := make([]jpf.ToolSchema, len(a.toolCatalogue))
	for i, t := range a.toolCatalogue {
		schemas[i] = t.Schema
	}
	return schemas
}

func (a *reactAgent) getMessagesForLLM() []jpf.Message {
	llmMessages := []jpf.Message{}

	systemMessage := a.systemMessage()
	headState := a.headStateMessage()

	// EmbedAfterSystem folds the head state into the system prompt, so there is
	// no standalone head state message to place.
	if headState != nil && a.headStatePlacement == EmbedAfterSystem {
		sys, _ := systemMessage.(jpf.SystemMessage)
		dev := headState.(jpf.DeveloperMessage)
		systemMessage = jpf.SystemMessage{Content: sys.Content + "\n\n" + dev.Content}
		headState = nil
	}

	if systemMessage != nil {
		llmMessages = append(llmMessages, systemMessage)
	}
	if headState != nil && a.headStatePlacement == DeveloperBeforeConversation {
		llmMessages = append(llmMessages, headState)
	}

	llmMessages = append(llmMessages, a.session.CoreMessages...)

	if headState != nil && a.headStatePlacement == DeveloperAfterConversation {
		llmMessages = append(llmMessages, headState)
	}

	return llmMessages
}

func (a *reactAgent) getActiveSkills() []jpf.Skill {
	activeSkills := make([]jpf.Skill, 0)
	for _, name := range a.session.ActiveSkillNames {
		s, err := a.lookupSkill(name)
		if err != nil {
			continue
		}
		activeSkills = append(activeSkills, s)
	}
	return activeSkills
}

func (a *reactAgent) lookupSkill(name string) (jpf.Skill, error) {
	for _, s := range a.skillCatalogue {
		if s.Name == name {
			return s, nil
		}
	}
	return jpf.Skill{}, fmt.Errorf("could not find skill with name '%s'", name)
}

func (a *reactAgent) headStateMessage() jpf.Message {
	// Safety guard: the rendering and fragment logic below assumes a non-nil map.
	if a.session.PromptFragments == nil {
		a.session.PromptFragments = make(map[string]string)
	}

	headState := &strings.Builder{}

	if len(a.skillCatalogue) > 0 {
		activeSkills := a.getActiveSkills()
		headState.WriteString("# Skills\nBelow are the activated and non-activated skills. These are up to date - activating / deactivating a skill will change it in this message. If you need a new skill, activate it. On the other hand, if you no longer need a skill, deactivate it to save context.\n## Active Skills\n")
		for _, s := range activeSkills {
			fmt.Fprintf(headState, "Skill '%s'\n%s\n\n", s.Name, s.Content)
		}
		headState.WriteString("# Available Skills\nBelow is a list of every skill that is avaiable for you to activate.\n")
		for _, s := range a.skillCatalogue {
			if slices.Contains(a.session.ActiveSkillNames, s.Name) {
				continue
			}
			fmt.Fprintf(headState, "Skill '%s', activate when: %s\n", s.Name, s.Description)
		}
	}

	keysOrdered := slices.Collect(maps.Keys(a.session.PromptFragments))
	slices.Sort(keysOrdered)
	if len(keysOrdered) > 0 {
		if headState.Len() > 0 {
			headState.WriteString("\n\n")
		}
		headState.WriteString("# Extra context")
		for _, key := range keysOrdered {
			fmt.Fprintf(headState, "\n\n> Below is extra context with key `%s`\n\n%s", key, a.session.PromptFragments[key])
		}
	}

	if headState.Len() == 0 {
		return nil
	}
	return jpf.DeveloperMessage{Content: headState.String()}
}

func (a *reactAgent) systemMessage() jpf.Message {
	prompt := fmt.Sprintf("# Instructions\n%s\n\n# Personality\n%s\n\n# Task\n%s", a.session.AgentPrompt, a.session.PersonalityPrompt, a.session.TaskPrompt)
	return jpf.SystemMessage{Content: prompt}
}

func (a *reactAgent) applyFragmentsFromCallbacks() {
	for _, cb := range a.headStateCallbacks {
		res := cb(a.Session())
		a.applyFragments(res)
	}
}

func (a *reactAgent) applyFragments(frags map[string]string) {
	if a.session.PromptFragments == nil {
		a.session.PromptFragments = make(map[string]string)
	}
	if frags == nil {
		return
	}
	for k, v := range frags {
		if v == "" {
			delete(a.session.PromptFragments, k)
		} else {
			a.session.PromptFragments[k] = v
		}
	}
}
