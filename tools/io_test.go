package tools

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestResolveAndCheckPath(t *testing.T) {
	root := t.TempDir()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		input   string
		want    string // path relative to absRoot; "" means absRoot itself. Ignored when wantErr.
		wantErr bool
	}{
		{name: "relative child", input: "foo/bar.txt", want: "foo/bar.txt"},
		{name: "relative dot is the root", input: ".", want: ""},
		{name: "relative dotdot back inside is fine", input: "a/../b", want: "b"},
		{name: "trailing slash is cleaned", input: "foo/", want: "foo"},
		{name: "absolute root itself", input: absRoot, want: ""},
		{name: "absolute child", input: filepath.Join(absRoot, "a", "b"), want: "a/b"},
		{name: "relative dotdot escapes", input: "../outside", wantErr: true},
		{name: "relative nested dotdot escapes", input: "a/b/../../../outside", wantErr: true},
		{name: "absolute parent escapes", input: filepath.Dir(absRoot), wantErr: true},
		{name: "absolute sibling escapes", input: filepath.Join(filepath.Dir(absRoot), "sibling"), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveAndCheckPath(root, tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got path %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := absRoot
			if tt.want != "" {
				want = filepath.Join(absRoot, filepath.FromSlash(tt.want))
			}
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

// A directory symlinked into the workspace should be usable as if its contents
// really lived there - the check is lexical and does not resolve symlinks.
func TestResolveAndCheckPathFollowsSymlinkedDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privilege on windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "data.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}

	got, err := resolveAndCheckPath(root, "linked/data.txt")
	if err != nil {
		t.Fatalf("symlinked dir should be usable: %v", err)
	}
	if data, err := os.ReadFile(got); err != nil || string(data) != "hello" {
		t.Fatalf("reading through the symlink failed: data=%q err=%v", data, err)
	}
}

// ".." is resolved lexically against the symlink's location in the workspace,
// not its target, so the agent cannot escape through a symlinked directory.
func TestResolveAndCheckPathCannotEscapeThroughSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privilege on windows")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveAndCheckPath(root, "linked/../../secret"); err == nil {
		t.Fatal("expected an error escaping through a symlinked directory")
	}
}

func TestReadFileCapped(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, n int) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, bytes.Repeat([]byte("x"), n), 0644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	tests := []struct {
		name    string
		size    int
		limit   int
		wantErr bool
	}{
		{name: "under limit", size: 10, limit: 100},
		{name: "exactly at limit", size: 100, limit: 100},
		{name: "one over limit", size: 101, limit: 100, wantErr: true},
		{name: "far over limit", size: 100_000, limit: 100, wantErr: true},
		{name: "empty file", size: 0, limit: 100},
		{name: "zero limit empty file", size: 0, limit: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readFileCapped(write(tt.name, tt.size), tt.limit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error for size %d limit %d", tt.size, tt.limit)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.size {
				t.Fatalf("got %d bytes, want %d", len(got), tt.size)
			}
		})
	}
}

func TestReadFileCappedMissingFile(t *testing.T) {
	if _, err := readFileCapped(filepath.Join(t.TempDir(), "nope"), 100); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func callReadFile(t *testing.T, tool jpf.Tool, args jpf.ToolArgs) (string, error) {
	t.Helper()
	res, err := tool.Call(context.Background(), args)
	return res.Content, err
}

func TestReadFileToolDefaultReadsWholeFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileToolOffsetAndCount(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "offset": 6, "count": 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "world" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileToolOffsetOnlyReadsToEnd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "offset": 6})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "world" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileToolCountOnlyReadsFromStart(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "count": 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileToolOffsetBeyondEndReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "offset": 1000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileToolCountClampedToEndOfFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "offset": 2, "count": 1000})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "llo" {
		t.Fatalf("got %q", got)
	}
}

// The size limit should cap what is returned per call, not the whole file, so
// a window into an oversized file should still succeed.
func TestReadFileToolWindowBypassesWholeFileSizeLimit(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.txt"), bytes.Repeat([]byte("x"), 1000), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 10)

	got, err := callReadFile(t, tool, jpf.ToolArgs{"path": "big.txt", "offset": 500, "count": 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != string(bytes.Repeat([]byte("x"), 10)) {
		t.Fatalf("got %d bytes, want 10", len(got))
	}
}

func TestReadFileToolWindowLargerThanSizeLimitErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.txt"), bytes.Repeat([]byte("x"), 1000), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 10)

	if _, err := callReadFile(t, tool, jpf.ToolArgs{"path": "big.txt", "count": 11}); err == nil {
		t.Fatal("expected an error requesting more than the size limit")
	}
	// Defaulting to "read to end" on an oversized file should also error.
	if _, err := callReadFile(t, tool, jpf.ToolArgs{"path": "big.txt"}); err == nil {
		t.Fatal("expected an error reading a whole oversized file with no count")
	}
}

func TestReadFileToolNegativeOffsetOrCountErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileReadTool(root, 1000)

	if _, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "offset": -1}); err == nil {
		t.Fatal("expected an error for negative offset")
	}
	if _, err := callReadFile(t, tool, jpf.ToolArgs{"path": "f.txt", "count": -1}); err == nil {
		t.Fatal("expected an error for negative count")
	}
}

func TestModifyFileToolReturnsTouchedByteRange(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("the cat sat"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileModifyTool(root, 1000)

	res, err := tool.Call(context.Background(), jpf.ToolArgs{"path": "f.txt", "old_text": "cat", "new_text": "dog"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "cat" starts at byte 4; "dog" is also 3 bytes, so it occupies [4, 7).
	if !strings.Contains(res.Content, "4-7") {
		t.Fatalf("expected touched range 4-7 in result, got %q", res.Content)
	}
	got, err := os.ReadFile(filepath.Join(root, "f.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "the dog sat" {
		t.Fatalf("got %q", got)
	}
}

func TestModifyFileToolReturnsTouchedByteRangeWhenFillingEmptyFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.txt"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	tool := newFileModifyTool(root, 1000)

	res, err := tool.Call(context.Background(), jpf.ToolArgs{"path": "f.txt", "old_text": "", "new_text": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Content, "0-5") {
		t.Fatalf("expected touched range 0-5 in result, got %q", res.Content)
	}
}
