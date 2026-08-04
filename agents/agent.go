package agents

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"

	"github.com/JoshPattman/jpf"
	"github.com/JoshPattman/jpf/internal/utils"
)

const defaultAgentInstruction = `You are a ReAct agent.
You will call tools until your job is complete, then you will provide a final response with no further tool calls to indicate you are finished iterating until the next task / message.`

const defaultAgentPersonality = `Your name is simply 'AI Assistant'.`

func NewAgent(maxIterations int, model jpf.Model) *Agent {
	a := &Agent{
		defaultAgentInstruction,
		defaultAgentPersonality,
		nil,
		nil,
		nil,
		nil,
		maxIterations,
		model,
	}
	a.SetCoreMessages(nil)
	a.SetSkills(nil)
	a.SetTools(nil)
	return a
}

type Agent struct {
	agentInstruction       string
	personalityInstruction string
	coreMessages           []jpf.Message
	tools                  []Tool
	skills                 []Skill
	activeSkills           []string
	maxIterations          int
	model                  jpf.Model
}

func (a *Agent) CoreMessages() iter.Seq[jpf.Message] {
	return slices.Values(a.coreMessages)
}

func (a *Agent) Run(ctx context.Context, query string, messageCallback func(jpf.Message)) error {
	if messageCallback == nil {
		messageCallback = func(jpf.Message) {}
	}
	a.coreMessages = append(a.coreMessages, jpf.UserMessage{Content: query})
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

func (a *Agent) SetTools(tools []Tool) {
	a.tools = slices.Concat(a.getBuiltinTools(), slices.Clone(tools))
}

func (a *Agent) SetSkills(skills []Skill) {
	a.skills = slices.Clone(skills)
	nextActiveSkills := make([]string, 0)
	for _, s := range a.activeSkills {
		_, err := a.lookupSkill(s)
		if err == nil {
			nextActiveSkills = append(nextActiveSkills, s)
		}
	}
	a.activeSkills = nextActiveSkills
}

func (a *Agent) SetCoreMessages(msgs []jpf.Message) {
	a.coreMessages = slices.Clone(msgs)
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
			nameAny, ok := m["skill_name"]
			if !ok {
				return "", fmt.Errorf("must provide skill_name")
			}
			name, ok := nameAny.(string)
			if !ok {
				return "", fmt.Errorf("skill_name must be a string")
			}
			if slices.Contains(a.activeSkills, name) {
				return "", fmt.Errorf("skill '%s' is already active", name)
			}
			skill, err := a.lookupSkill(name)
			if err != nil {
				return "", err
			}
			a.activeSkills = append(a.activeSkills, skill.Name)
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
			if !slices.Contains(a.activeSkills, name) {
				return "", fmt.Errorf("skill '%s' is not currently active", name)
			}
			skill, err := a.lookupSkill(name)
			if err != nil {
				return "", err
			}
			a.activeSkills = slices.DeleteFunc(a.activeSkills, func(s string) bool { return s == skill.Name })
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
	a.coreMessages = append(a.coreMessages, response.Message)
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

	for i, call := range action.ToolCalls {
		result, err := tools[i].Call(ctx, call.Args)
		msg := jpf.ToolResultMessage{CallID: call.ID}
		if err != nil {
			msg.Result = fmt.Sprintf("The tool call failed with error: %s", err.Error())
		} else {
			msg.Result = result
		}
		a.coreMessages = append(a.coreMessages, msg)
		messageCallback(msg)
	}

	return nil
}

func (a *Agent) lookupTool(name string) (Tool, error) {
	for _, t := range a.tools {
		if t.Schema.Name == name {
			return t, nil
		}
	}
	return Tool{}, fmt.Errorf("could not find tool with name '%s'", name)
}

func (a *Agent) toolSchemas() []jpf.ToolSchema {
	schemas := make([]jpf.ToolSchema, len(a.tools))
	for i, t := range a.tools {
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
	llmMessages = append(llmMessages, a.coreMessages...)
	return llmMessages
}

func (a *Agent) getActiveSkills() []Skill {
	activeSkills := make([]Skill, 0)
	for _, name := range a.activeSkills {
		s, err := a.lookupSkill(name)
		if err != nil {
			continue
		}
		activeSkills = append(activeSkills, s)
	}
	return activeSkills
}

func (a *Agent) lookupSkill(name string) (Skill, error) {
	for _, s := range a.skills {
		if s.Name == name {
			return s, nil
		}
	}
	return Skill{}, fmt.Errorf("could not find skill with name '%s'", name)
}

func (a *Agent) headStateMessage() jpf.Message {
	if len(a.skills) == 0 {
		return nil
	}
	activeSkills := a.getActiveSkills()
	headState := &strings.Builder{}
	headState.WriteString("# Skills\nBelow are the activated and non-activated skills. These are up to date - activating / deactivating a skill will change it in this message. If you need a new skill, activate it. On the other hand, if you no longer need a skill, deactivate it to save context.\n## Active Skills\n")
	for _, s := range activeSkills {
		fmt.Fprintf(headState, "Skill '%s'\n%s\n\n", s.Name, s.Content)
	}
	headState.WriteString("# Available Skills\nBelow is a list of every skill that is avaiable for you to activate.\n")
	for _, s := range a.skills {
		if slices.Contains(a.activeSkills, s.Name) {
			continue
		}
		fmt.Fprintf(headState, "Skill '%s', activate when: %s\n", s.Name, s.Description)
	}
	return jpf.DeveloperMessage{Content: headState.String()}
}

func (a *Agent) systemMessage() jpf.Message {
	prompt := fmt.Sprintf("# Instructions\n%s\n\n# Personality & Task\n%s", a.agentInstruction, a.personalityInstruction)
	return jpf.SystemMessage{Content: prompt}
}
