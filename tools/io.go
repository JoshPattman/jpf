package tools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/JoshPattman/jpf"
)

type IOToolLevel uint8

const (
	// Readonly access to the filesystem.
	ReadFSLevel IOToolLevel = iota
	// Read and write access to the filesystem.
	WriteFSLevel
)

// BuildIOTools builds the set of filesystem tools for an agent to interact with
// its workspace. All tools are sandboxed to workspaceRoot. The tools included
// depend on level: ReadFSLevel gives read-only tools, WriteFSLevel adds the
// tools that create, modify, and delete files and directories.
func BuildIOTools(level IOToolLevel, workspaceRoot string) []jpf.Tool {
	fileSizeLimit := 50000
	readDirNumLimit := 250
	ts := []jpf.Tool{
		newWorkspaceRootTool(workspaceRoot),
		newFileReadTool(workspaceRoot, fileSizeLimit),
		newDirReadTool(workspaceRoot, readDirNumLimit),
	}
	if level == WriteFSLevel {
		ts = append(
			ts,
			newFileCreateTool(workspaceRoot),
			newFileModifyTool(workspaceRoot, fileSizeLimit),
			newFileDeleteTool(workspaceRoot),
			newDirCreateTool(workspaceRoot),
			newDirDeleteTool(workspaceRoot),
		)
	}
	return ts
}

// resolveAndCheckPath resolves path to an absolute path (relative paths are
// taken relative to the workspace root) and verifies that the result is the
// workspace root itself or somewhere beneath it. It returns an error if the
// path escapes the workspace root.
//
// The check is purely lexical: symlinks are deliberately not resolved, so a
// directory symlinked into the workspace can be used by the agent as if its
// contents really lived there. The one consequence is that ".." is resolved
// against a symlink's location in the workspace rather than its target, so the
// agent cannot walk out of the workspace through a symlinked directory.
func resolveAndCheckPath(workspaceRoot, path string) (string, error) {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve the workspace root: %w", err)
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(absRoot, path)
	}
	abs := filepath.Clean(path)
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("that path is outside the workspace root")
	}
	return abs, nil
}

// readFileCapped reads the file at path into a pre-allocated buffer of limit+1
// bytes. If the file does not fit, it returns an error without reading the rest
// of the file, so at most limit+1 bytes are ever held in memory.
func readFileCapped(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, limit+1)
	n, err := io.ReadFull(f, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return nil, err
	}
	if n > limit {
		return nil, fmt.Errorf("that file is larger than the maximum size limit of %d bytes, so it cannot be read", limit)
	}
	return buf[:n], nil
}

func newFileReadTool(workspaceRoot string, sizeLimit int) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
			contents, err := readFileCapped(path, sizeLimit)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to read that file"), err)
			}
			return jpf.ToolResult{
				Content: string(contents),
			}, nil
		},
	}
}

func newDirReadTool(workspaceRoot string, numLimit int) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
			entries, err := os.ReadDir(path)
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

func newFileCreateTool(workspaceRoot string) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
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

func newFileDeleteTool(workspaceRoot string) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
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

func newFileModifyTool(workspaceRoot string, sizeLimit int) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
			oldText := ta.RequiredString("old_text")
			newText := ta.RequiredString("new_text")

			contents, err := readFileCapped(path, sizeLimit)
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
			if len(updated) > sizeLimit {
				return jpf.ToolResult{}, fmt.Errorf("the modified file would be larger than the maximum size limit of %d bytes", sizeLimit)
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

func newDirCreateTool(workspaceRoot string) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
			if err := os.MkdirAll(path, 0755); err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to create that directory"), err)
			}
			return jpf.ToolResult{
				Content: fmt.Sprintf("created directory %s", path),
			}, nil
		},
	}
}

func newDirDeleteTool(workspaceRoot string) jpf.Tool {
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
			path, err := resolveAndCheckPath(workspaceRoot, ta.RequiredString("path"))
			if err != nil {
				return jpf.ToolResult{}, err
			}
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

func newWorkspaceRootTool(workspaceRoot string) jpf.Tool {
	return jpf.Tool{
		Schema: jpf.ToolSchema{
			Name:        "workspace_root",
			Description: "get the absolute path of the workspace root directory, dumping the result in your context. All file and directory paths you use must resolve to somewhere inside this directory.",
			Params:      []jpf.ToolParam{},
		},
		Call: func(ctx context.Context, ta jpf.ToolArgs) (jpf.ToolResult, error) {
			absRoot, err := filepath.Abs(workspaceRoot)
			if err != nil {
				return jpf.ToolResult{}, errors.Join(fmt.Errorf("failed to resolve the workspace root"), err)
			}
			return jpf.ToolResult{
				Content: absRoot,
			}, nil
		},
	}
}
