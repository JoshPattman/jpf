package agents

import (
	"context"

	"github.com/JoshPattman/jpf"
)

type Tool struct {
	Schema jpf.ToolSchema
	Call   func(context.Context, map[string]any) (string, error)
}
