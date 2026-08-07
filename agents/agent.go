package agents

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

func NewAgentInstance(model jpf.Model) *AgentInstance {
	a := &AgentInstance{
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

type AgentInstance struct {
	session        AgentSession
	toolCatalogue  []Tool
	skillCatalogue []Skill
	maxIterations  int
	model          jpf.Model
}

func (a *AgentInstance) Session() AgentSession {
	return a.session.Clone()
}

func (a *AgentInstance) SetSession(sess AgentSession) {
	a.session = sess.Clone()
}

func (a *AgentInstance) SetMaxIterations(n int) {
	a.maxIterations = n
}

func (a *AgentInstance) SetToolCatalogue(tools []Tool) {
	a.toolCatalogue = slices.Concat(a.getBuiltinTools(), slices.Clone(tools))
}

func (a *AgentInstance) SetSkillCatalogue(skills []Skill) {
	a.skillCatalogue = slices.Clone(skills)
}

func (a *AgentInstance) Run(ctx context.Context, query string, messageCallback func(jpf.Message)) error {
	if messageCallback == nil {
		messageCallback = func(jpf.Message) {}
	}
	a.deactivateMissingActiveSkills()
	a.session.CoreMessages = append(a.session.CoreMessages, jpf.UserMessage{Content: query})
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
	}
	return nil
}

func (a *AgentInstance) deactivateMissingActiveSkills() {
	nextActiveSkills := make([]string, 0)
	for _, s := range a.session.ActiveSkillNames {
		_, err := a.lookupSkill(s)
		if err == nil {
			nextActiveSkills = append(nextActiveSkills, s)
		}
	}
	a.session.ActiveSkillNames = nextActiveSkills
}

func (a *AgentInstance) getBuiltinTools() []Tool {
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
			nameAny, ok := m["skill_name"]
			if !ok {
				return "", fmt.Errorf("must provide skill_name")
			}
			name, ok := nameAny.(string)
			if !ok {
				return "", fmt.Errorf("skill_name must be a string")
			}
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
			nameAny, ok := m["skill_name"]
			if !ok {
				return "", fmt.Errorf("must provide skill_name")
			}
			name, ok := nameAny.(string)
			if !ok {
				return "", fmt.Errorf("skill_name must be a string")
			}
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

func (a *AgentInstance) determineNextAction(ctx context.Context, messageCallback func(jpf.Message)) (jpf.AssistantMessage, error) {
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

func (a *AgentInstance) executeToolCalls(ctx context.Context, messageCallback func(jpf.Message), action jpf.AssistantMessage) error {
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

	for i, call := range action.ToolCalls {
		result, err := tools[i].Call(ctx, call.Args)
		msg := jpf.ToolResultMessage{CallID: call.ID}
		if err != nil {
			msg.Result = fmt.Sprintf("The tool call failed with error: %s", err.Error())
		} else {
			msg.Result = result
		}
		a.session.CoreMessages = append(a.session.CoreMessages, msg)
		messageCallback(msg)
	}

	return nil
}

func (a *AgentInstance) lookupTool(name string) (Tool, error) {
	for _, t := range a.toolCatalogue {
		if t.Schema.Name == name {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("could not find tool with name '%s'", name)
}

func (a *AgentInstance) toolSchemas() []jpf.ToolSchema {
	schemas := make([]jpf.ToolSchema, len(a.toolCatalogue))
	for i, t := range a.toolCatalogue {
		schemas[i] = t.Schema
	}
	return schemas
}

func (a *AgentInstance) getMessagesForLLM() []jpf.Message {
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

func (a *AgentInstance) getActiveSkills() []Skill {
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

func (a *AgentInstance) lookupSkill(name string) (Skill, error) {
	for _, s := range a.skillCatalogue {
		if s.Name == name {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("could not find skill with name '%s'", name)
}

func (a *AgentInstance) headStateMessage() jpf.Message {
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

func (a *AgentInstance) systemMessage() jpf.Message {
	prompt := fmt.Sprintf("# Instructions\n%s\n\n# Personality\n%s\n\n# Task\n%s", a.session.AgentPrompt, a.session.PersonalityPrompt, a.session.TaskPrompt)
	return jpf.SystemMessage{Content: prompt}
}
