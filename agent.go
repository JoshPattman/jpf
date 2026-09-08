package jpf

import (
	"context"
	"maps"
	"slices"
)

type Agent interface {
	Session() AgentSession
	SetSession(AgentSession)
	SetMaxIterations(int)
	SetToolCatalogue([]Tool)
	SetSkillCatalogue([]Skill)
	Run(context.Context, string, ...AgentResponseOpt) error
	Resume(context.Context, []DeferredCallResult, ...AgentResponseOpt) error
}

type Skill struct {
	Name        string
	Description string
	Content     string
}

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
	CoreMessages []Message
	// The current tool calls that have been deferred, with their validated args.
	CurrentDeferredToolCalls []DeferredToolCall
	// The names of the skills that should currently be active.
	ActiveSkillNames []string
	// PromptFragments are extra blocks of keyed text rendered in the head state,
	// ordered by key. A tool can add or overwrite a fragment by returning it in
	// its result; returning a key with an empty value removes that fragment.
	PromptFragments map[string]string
}

func (a AgentSession) Clone() AgentSession {
	return AgentSession{
		a.AgentPrompt,
		a.TaskPrompt,
		a.PersonalityPrompt,
		slices.Clone(a.CoreMessages),
		slices.Clone(a.CurrentDeferredToolCalls),
		slices.Clone(a.ActiveSkillNames),
		maps.Clone(a.PromptFragments),
	}
}

func DefaultAgentSession() AgentSession {
	return AgentSession{
		AgentPrompt:       defaultAgentPrompt,
		TaskPrompt:        defaultTaskPrompt,
		PersonalityPrompt: defaultPersonalityPrompt,
		PromptFragments:   make(map[string]string),
	}
}

type AgentStreamer interface {
	OnMessageComplete(Message)
}

type AgentResponseKwargs struct {
	Streamer AgentStreamer
}

type AgentResponseOpt func(*AgentResponseKwargs)

func WithStreamActions(streamer AgentStreamer) AgentResponseOpt {
	return func(ark *AgentResponseKwargs) {
		ark.Streamer = streamer
	}
}

type nullStreamer struct{}

func (nullStreamer) OnMessageComplete(Message) {}

func GetAgentResponseKwargs(opts []AgentResponseOpt) AgentResponseKwargs {
	kwargs := AgentResponseKwargs{}
	for _, o := range opts {
		o(&kwargs)
	}
	if kwargs.Streamer == nil {
		kwargs.Streamer = nullStreamer{}
	}
	return kwargs
}
