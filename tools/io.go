package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JoshPattman/jpf"
)

func NewReadonlyFSTools() []jpf.Tool {
	return []jpf.Tool{
		NewFileReadTool(10000),
		NewDirReadTool(250),
		NewPWDTool(),
	}
}

func NewFileReadTool(sizeLimit int) jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "read_file",
			Description: "read the contents of a file on disk, dumping the result in your context.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the file to read",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			contents, err := os.ReadFile(ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to read that file"), err)
			}
			if len(contents) > sizeLimit {
				return jpf.ToolResult{}, fmt.Errorf("that file is larger than the maximum size limit, so you cannot read it (%d > %d).", len(contents), sizeLimit)
			}
			return jpf.ToolResult{
				Content: string(contents),
			}, nil
		},
	}
}

func NewDirReadTool(numLimit int) jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "read_dir",
			Description: "list the contents of a directory on disk, dumping the result in your context.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the directory to read",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			entries, err := os.ReadDir(ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to read that directory"), err)
			}
			if len(entries) > numLimit {
				return jpf.ToolResult{}, fmt.Errorf("that directory has more entries than the maximum size limit, so you cannot read it (%d > %d).", len(entries), numLimit)
			}
			lines := make([]string, len(entries))
			for i, entry := range entries {
				if entry.IsDir() {
					lines[i] = entry.Name() + "/"
				} else {
					lines[i] = entry.Name()
				}
			}
			return jpf.ToolResult{
				Content: strings.Join(lines, "\n"),
			}, nil
		},
	}
}

func NewPWDTool() jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "pwd",
			Description: "get the current working directory, dumping the result in your context.",
			Params:      []jpf.ToolParam{},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			wd, err := os.Getwd()
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to get the working directory"), err)
			}
			return jpf.ToolResult{
				Content: wd,
			}, nil
		},
	}
}
