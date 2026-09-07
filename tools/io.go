package tools

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/JoshPattman/jpf"
)

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

func NewFileCreateTool() jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "create_file",
			Description: "create a new empty file on disk. Fails if the file already exists.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the file to create",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			path := ta.RequiredString("path")
			f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to create that file"), err)
			}
			if err := f.Close(); err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to create that file"), err)
			}
			return jpf.ToolResult{
				Content: fmt.Sprintf("created empty file %s", path),
			}, nil
		},
	}
}

func NewFileDeleteTool() jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "delete_file",
			Description: "delete a file on disk.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the file to delete",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			path := ta.RequiredString("path")
			info, err := os.Stat(path)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to delete that file"), err)
			}
			if info.IsDir() {
				return jpf.ToolResult{}, fmt.Errorf("that path is a directory, not a file")
			}
			if err := os.Remove(path); err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to delete that file"), err)
			}
			return jpf.ToolResult{
				Content: fmt.Sprintf("deleted file %s", path),
			}, nil
		},
	}
}

func NewFileModifyTool() jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "modify_file",
			Description: "modify an existing file on disk by replacing a snippet of its text. The old text must appear exactly once in the file, otherwise this fails. To fill an empty file, pass an empty string as the old text.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the file to modify",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
				{
					Name:        "old_text",
					Description: "the exact existing text to replace. Must match exactly once in the file. Pass an empty string to fill an empty file.",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
				{
					Name:        "new_text",
					Description: "the text to replace the old text with",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			path := ta.RequiredString("path")
			oldText := ta.RequiredString("old_text")
			newText := ta.RequiredString("new_text")

			contents, err := os.ReadFile(path)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to read that file"), err)
			}

			var updated string
			if oldText == "" {
				if len(contents) != 0 {
					return jpf.ToolResult{}, fmt.Errorf("old text is empty but that file is not empty, so there is nothing to fill")
				}
				updated = newText
			} else {
				count := strings.Count(string(contents), oldText)
				if count != 1 {
					return jpf.ToolResult{}, fmt.Errorf("old text must appear exactly once in that file, but it appears %d times", count)
				}
				updated = strings.Replace(string(contents), oldText, newText, 1)
			}

			info, err := os.Stat(path)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to modify that file"), err)
			}
			if err := os.WriteFile(path, []byte(updated), info.Mode().Perm()); err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to modify that file"), err)
			}
			return jpf.ToolResult{
				Content: fmt.Sprintf("modified file %s", path),
			}, nil
		},
	}
}

func NewDirCreateTool() jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "create_dir",
			Description: "create a directory on disk, including any necessary parent directories.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the directory to create",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			path := ta.RequiredString("path")
			if err := os.MkdirAll(path, 0755); err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to create that directory"), err)
			}
			return jpf.ToolResult{
				Content: fmt.Sprintf("created directory %s", path),
			}, nil
		},
	}
}

func NewDirDeleteTool() jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "delete_dir",
			Description: "delete an empty directory on disk. Fails if the directory is not empty.",
			Params: []jpf.ToolParam{
				{
					Name:        "path",
					Description: "the path of the directory to delete",
					Type:        jpf.ToolParamString,
					Required:    true,
				},
			},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			path := ta.RequiredString("path")
			info, err := os.Stat(path)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to delete that directory"), err)
			}
			if !info.IsDir() {
				return jpf.ToolResult{}, fmt.Errorf("that path is a file, not a directory")
			}
			// os.Remove only deletes a directory if it is empty.
			if err := os.Remove(path); err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to delete that directory"), err)
			}
			return jpf.ToolResult{
				Content: fmt.Sprintf("deleted directory %s", path),
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
