package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/JoshPattman/jpf"
)

// BuildWorkingMemoryUtils builds a tool that lets an agent maintain a
// persistent block of free-form working memory, plus the head state callback
// that renders it into the agent's prompt fragments.
func BuildWorkingMemoryUtils() ([]jpf.Tool, []func(jpf.AgentSession) map[string]string) {
	memString := ""
	mem := &memString

	updateMemoryTool := jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "update_memory",
			Description: "update your persistent working memory by replacing a snippet of its text. The old text must appear exactly once in the memory, otherwise this fails. To fill empty memory, pass an empty string as the old text.",
			Params: []jpf.ToolParam{
				{
					Name:        "old_text",
					Description: "the exact existing text to replace. Must match exactly once in the memory. Pass an empty string to fill empty memory.",
					Type:        jpf.ToolParamString,
				},
				{
					Name:        "new_text",
					Description: "the text to replace the old text with",
					Type:        jpf.ToolParamString,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			oldText := ta.String("old_text")
			newText := ta.String("new_text")

			var updated string
			if oldText == "" {
				if len(*mem) != 0 {
					return jpf.ToolResult{}, fmt.Errorf("old text is empty but working memory is not empty, so there is nothing to fill")
				}
				updated = newText
			} else {
				count := strings.Count(*mem, oldText)
				if count != 1 {
					return jpf.ToolResult{}, fmt.Errorf("old text must appear exactly once in working memory, but it appears %d times", count)
				}
				updated = strings.Replace(*mem, oldText, newText, 1)
			}
			*mem = updated

			return jpf.ToolResult{
				Content: "updated working memory",
			}, nil
		},
	}

	return []jpf.Tool{
			updateMemoryTool,
		}, []func(jpf.AgentSession) map[string]string{
			func(as jpf.AgentSession) map[string]string {
				return map[string]string{
					"working_memory": *mem,
				}
			},
		}
}
