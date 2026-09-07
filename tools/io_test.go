package tools

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
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
