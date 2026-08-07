package agents

import (
	"slices"

	"github.com/JoshPattman/jpf"
)

const defaultAgentPrompt = `You are a ReAct agent.
You will call tools until your job is complete, then you will provide a final response with no further tool calls to indicate you are finished iterating until the next task / message.`

const defaultTaskPrompt = "You are a generalist and will complete whichever task the user requires."

const defaultPersonalityPrompt = `Your name is simply 'AI Assistant'. You behave with a neutral but helpful personality.`

// An agent session describes the state of an agent at the current point in time,
// avoiding any specific logic or details of skills.
type AgentSession struct {
	// The prompt that tells the agent how to interact with the agent loop and react loop,
	// usually does not need to be changed.
	AgentPrompt string
	// The prompt telling the agent specifically about its task,
	// if not specified will tell the agent to be a generalist.
	TaskPrompt string
	// The prompt telling the agent how to act / speak,
	// if not specified will speak neutrally and refer to itself as AI Assistant.
	PersonalityPrompt string
	// The messages, excluding system and other special messages.
	CoreMessages []jpf.Message
	// The names of the skills that should currently be active.
	ActiveSkillNames []string
}

func (a AgentSession) Clone() AgentSession {
	return AgentSession{
		a.AgentPrompt,
		a.TaskPrompt,
		a.PersonalityPrompt,
		slices.Clone(a.CoreMessages),
		slices.Clone(a.ActiveSkillNames),
	}
}

func DefaultSession() AgentSession {
	return AgentSession{
		AgentPrompt:       defaultAgentPrompt,
		TaskPrompt:        defaultTaskPrompt,
		PersonalityPrompt: defaultPersonalityPrompt,
	}
}
