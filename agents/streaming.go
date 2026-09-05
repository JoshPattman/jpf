package agents

import "github.com/JoshPattman/jpf"

type AgentStreamer interface {
	OnMessageComplete(jpf.Message)
}

type AgentStreamerBase struct{}

func (*AgentStreamerBase) OnMessageComplete(jpf.Message) {}
