package tools

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/JoshPattman/jpf"
)

// NewRunBashCommandTool creates a tool to allow the llm to run arbritrary
// bash commands. Note that this is not sandboxed! The LLM will basically
// have complete access to your PC, use this only in containers / VMs!
func NewRunBashCommandTool(workspaceRoot string) jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "run_bash_command",
			Description: "run a shell command via 'bash -c' from the workspace root directory, dumping its combined stdout and stderr into your context. A non-zero exit code is reported as an error.",
			Params: []jpf.ToolParam{
				{
					Name:        "command",
					Description: "the command to run, as a single string passed to 'bash -c'",
					Type:        jpf.ToolParamString,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			command := ta.String("command")
			cmd := exec.CommandContext(ctx, "bash", "-c", command)
			cmd.Dir = workspaceRoot
			output, err := cmd.CombinedOutput()
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("that command failed: %s", strings.TrimSpace(string(output))), err)
			}
			return jpf.ToolResult{
				Content: string(output),
			}, nil
		},
	}
}
