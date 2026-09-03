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

func NewAgent(model jpf.Model) *Agent {
	a := &Agent{
		DefaultSession(),
		nil,
		nil,
		20,
		model,
	}
	a.SetSkillCatalogue(nil)
	a.SetToolCatalogue(nil)
	return a
}

type Agent struct {
	session        AgentSession
	toolCatalogue  []Tool
	skillCatalogue []Skill
	maxIterations  int
	model          jpf.Model
}

func (a *Agent) Session() AgentSession {
	return a.session.Clone()
}

func (a *Agent) SetSession(sess AgentSession) {
	a.session = sess.Clone()
}

func (a *Agent) SetMaxIterations(n int) {
	a.maxIterations = n
}

func (a *Agent) SetToolCatalogue(tools []Tool) {
	a.toolCatalogue = slices.Concat(a.getBuiltinTools(), slices.Clone(tools))
}

func (a *Agent) SetSkillCatalogue(skills []Skill) {
	a.skillCatalogue = slices.Clone(skills)
}

// Run the agent from a new message to add into the conversation.
// Should only be called if the agent is not currently awaiting deferred tool responses.
// May terminate because the agent is done, has hit max iterations, or is awaiting deferred tool responses.
func (a *Agent) Run(ctx context.Context, query string, messageCallback func(jpf.Message)) error {
	if len(a.CurrentDeferredToolCalls()) != 0 {
		return fmt.Errorf("cannot run an agent from fresh when it is awaiting deferred calls, please use resume instead")
	}
	if messageCallback == nil {
		messageCallback = func(jpf.Message) {}
	}
	msg := jpf.UserMessage{Content: query}
	a.session.CoreMessages = append(a.session.CoreMessages, msg)
	messageCallback(msg)
	return a.runOrResumeHelper(ctx, messageCallback)
}

// Resume the agent from a set of responses to deferred tool calls to add into the conversation.
// Should only be called if the agent is currently awaiting deferred tool responses.
// May terminate because the agent is done, has hit max iterations, or is awaiting deferred tool responses.
func (a *Agent) Resume(ctx context.Context, callResults []DeferredCallResponse, messageCallback func(jpf.Message)) error {
	defCalls := a.CurrentDeferredToolCalls()
	if len(defCalls) == 0 {
		return fmt.Errorf("cannot resume an agent when it is not awaiting deferred calls, please use run instead")
	}
	if len(defCalls) != len(callResults) {
		return fmt.Errorf("call results do not match the expected awaiting deferred calls")
	}
	if messageCallback == nil {
		messageCallback = func(jpf.Message) {}
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
	sess := a.Session()
	for _, result := range callResults {
		for i := len(sess.CoreMessages) - 1; i >= 0; i-- {
			resp, ok := sess.CoreMessages[i].(jpf.ToolResultMessage)
			if !ok {
				continue
			}
			if resp.CallID != result.CallID {
				continue
			}
			if result.Err != nil {
				resp.Result = fmt.Sprintf("The tool call failed with error: %s", result.Err.Error())
			} else {
				resp.Result = result.Result
			}
			sess.CoreMessages[i] = resp
			break
		}
	}
	// Run callback
	for i, msg := range slices.Backward(sess.CoreMessages) {
		_, ok := msg.(jpf.ToolResultMessage)
		if !ok {
			for j := i + 1; j < len(sess.CoreMessages); j++ {
				messageCallback(sess.CoreMessages[j])
			}
			break
		}
	}

	sess.CurrentDeferredToolCalls = nil
	a.SetSession(sess)
	return a.runOrResumeHelper(ctx, messageCallback)
}

func (a *Agent) runOrResumeHelper(ctx context.Context, messageCallback func(jpf.Message)) error {
	a.deactivateMissingActiveSkills()
	for range a.maxIterations {
		nextAction, err := a.determineNextAction(ctx, messageCallback)
		if err != nil {
			return utils.Wrap(err, "failed to determine next action")
		}
		if len(nextAction.ToolCalls) == 0 {
			break
		} else {
			err = a.executeToolCalls(ctx, messageCallback, nextAction)
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

type DeferredToolCall struct {
	ToolName string
	CallID   string
	Args     map[string]any
}

type DeferredCallResponse struct {
	CallID string
	Result string
	Err    error
}

func (a *Agent) CurrentDeferredToolCalls() []DeferredToolCall {
	return slices.Clone(a.Session().CurrentDeferredToolCalls)
}

func (a *Agent) deactivateMissingActiveSkills() {
	nextActiveSkills := make([]string, 0)
	for _, s := range a.session.ActiveSkillNames {
		_, err := a.lookupSkill(s)
		if err == nil {
			nextActiveSkills = append(nextActiveSkills, s)
		}
	}
	a.session.ActiveSkillNames = nextActiveSkills
}

func (a *Agent) getBuiltinTools() []Tool {
	activateSkillTool := Tool{
		Schema: jpf.ToolSchema{
			Name:        "activate_skill",
			Description: "activate a skill that is currently not active, causing the full skill body to show in all future context for you (in the head state)",
			Args: []jpf.ToolArg{
				{
					Name:        "skill_name",
					Description: "the name of the skill to activate",
					Type:        jpf.ToolArgString,
					Required:    true,
				},
			},
		},
		Call: func(_ context.Context, m map[string]any) (string, error) {
			name := RequiredArg[string](m, "skill_name")
			if slices.Contains(a.session.ActiveSkillNames, name) {
				return "", fmt.Errorf("skill '%s' is already active", name)
			}
			skill, err := a.lookupSkill(name)
			if err != nil {
				return "", err
			}
			a.session.ActiveSkillNames = append(a.session.ActiveSkillNames, skill.Name)
			return fmt.Sprintf("activated skill '%s'", skill.Name), nil
		},
	}

	deactivateSkillTool := Tool{
		Schema: jpf.ToolSchema{
			Name:        "deactivate_skill",
			Description: "deactivate a skill that is currently active, causing the full skill body to be removed in future calls (in the head state) - call this when you feel a skill is no longer useful to you and you are able to forget it for now",
			Args: []jpf.ToolArg{
				{
					Name:        "skill_name",
					Description: "the name of the skill to deactivate",
					Type:        jpf.ToolArgString,
					Required:    true,
				},
			},
		},
		Call: func(_ context.Context, m map[string]any) (string, error) {
			name := RequiredArg[string](m, "skill_name")
			if !slices.Contains(a.session.ActiveSkillNames, name) {
				return "", fmt.Errorf("skill '%s' is not currently active", name)
			}
			skill, err := a.lookupSkill(name)
			if err != nil {
				return "", err
			}
			a.session.ActiveSkillNames = slices.DeleteFunc(a.session.ActiveSkillNames, func(s string) bool { return s == skill.Name })
			return fmt.Sprintf("deactivated skill '%s'", skill.Name), nil
		},
	}
	return []Tool{
		activateSkillTool,
		deactivateSkillTool,
	}
}

func (a *Agent) determineNextAction(ctx context.Context, messageCallback func(jpf.Message)) (jpf.AssistantMessage, error) {
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

func (a *Agent) executeToolCalls(ctx context.Context, messageCallback func(jpf.Message), action jpf.AssistantMessage) error {
	tools := make([]Tool, len(action.ToolCalls))
	for i, call := range action.ToolCalls {
		tool, err := a.lookupTool(call.Tool)
		if err != nil {
			tools[i] = Tool{
				Call: func(ctx context.Context, m map[string]any) (string, error) {
					return "", fmt.Errorf("could not find tool with name '%s'", call.Tool)
				},
			}
		} else {
			tools[i] = tool
		}
	}

	deferredCalls := make([]DeferredToolCall, 0)
	toCallback := make([]jpf.ToolResultMessage, 0)

	for i, call := range action.ToolCalls {
		msg := jpf.ToolResultMessage{CallID: call.ID}

		args := maps.Clone(call.Args)
		err := validateAndFixArgsForSchema(args, tools[i].Schema)
		if err != nil {
			msg.Result = fmt.Sprintf("The tool call failed with error: %s", err.Error())
		} else if tools[i].Call == nil {
			msg.Result = ""
			deferredCalls = append(deferredCalls, DeferredToolCall{call.Tool, msg.CallID, args})
		} else {
			result, err := tools[i].Call(ctx, args)
			if err != nil {
				msg.Result = fmt.Sprintf("The tool call failed with error: %s", err.Error())
			} else {
				msg.Result = result
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

func (a *Agent) lookupTool(name string) (Tool, error) {
	for _, t := range a.toolCatalogue {
		if t.Schema.Name == name {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("could not find tool with name '%s'", name)
}

func (a *Agent) toolSchemas() []jpf.ToolSchema {
	schemas := make([]jpf.ToolSchema, len(a.toolCatalogue))
	for i, t := range a.toolCatalogue {
		schemas[i] = t.Schema
	}
	return schemas
}

func (a *Agent) getMessagesForLLM() []jpf.Message {
	llmMessages := []jpf.Message{}
	systemMessage := a.systemMessage()
	if systemMessage != nil {
		llmMessages = append(llmMessages, systemMessage)
	}
	headState := a.headStateMessage()
	if headState != nil {
		llmMessages = append(llmMessages, headState)
	}
	llmMessages = append(llmMessages, a.session.CoreMessages...)
	return llmMessages
}

func (a *Agent) getActiveSkills() []Skill {
	activeSkills := make([]Skill, 0)
	for _, name := range a.session.ActiveSkillNames {
		s, err := a.lookupSkill(name)
		if err != nil {
			continue
		}
		activeSkills = append(activeSkills, s)
	}
	return activeSkills
}

func (a *Agent) lookupSkill(name string) (Skill, error) {
	for _, s := range a.skillCatalogue {
		if s.Name == name {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("could not find skill with name '%s'", name)
}

func (a *Agent) headStateMessage() jpf.Message {
	if len(a.skillCatalogue) == 0 {
		return nil
	}
	activeSkills := a.getActiveSkills()
	headState := &strings.Builder{}
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
	return jpf.DeveloperMessage{Content: headState.String()}
}

func (a *Agent) systemMessage() jpf.Message {
	prompt := fmt.Sprintf("# Instructions\n%s\n\n# Personality\n%s\n\n# Task\n%s", a.session.AgentPrompt, a.session.PersonalityPrompt, a.session.TaskPrompt)
	return jpf.SystemMessage{Content: prompt}
}
