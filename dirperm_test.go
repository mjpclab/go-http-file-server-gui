package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"mjpclab.dev/ghfs/src/util"
)

// Cross-check hasPathPrefix against the ghfs function it mirrors.
func TestHasPathPrefixMatchesGhfs(t *testing.T) {
	cases := [][2]string{
		{"/a/b", "/a/b"},
		{"/a/b/c", "/a/b"},
		{"/a/bc", "/a/b"},
		{"/a/b", "/a/b/c"},
		{"/a", "/a/b"},
		{"/a/b/c/d", "/a"},
		{"/ab", "/a"},
		{"/a/b", "/"},
		{"/", "/"},
		{"/a/b", "/a/"},
	}
	for _, c := range cases {
		p, prefix := filepath.FromSlash(c[0]), filepath.FromSlash(c[1])
		got := hasPathPrefix(p, prefix)
		want := util.HasFsPrefixDir(p, prefix)
		if got != want {
			t.Errorf("hasPathPrefix(%q, %q) = %v, ghfs says %v", p, prefix, got, want)
		}
	}
}

func TestInheritanceAndAbbr(t *testing.T) {
	p := newDirPerms()
	ab := filepath.FromSlash("/a/b")
	abc := filepath.FromSlash("/a/b/c")
	abd := filepath.FromSlash("/a/b/d")
	ax := filepath.FromSlash("/a/x")

	p.set(ab, permUpload, true)
	p.set(abd, permArchive, true)

	if got := p.abbr(ab); got != "U" {
		t.Errorf("abbr(/a/b) = %q, want %q", got, "U")
	}
	if got := p.abbr(abc); got != "u" {
		t.Errorf("abbr(/a/b/c) = %q, want %q", got, "u")
	}
	if got := p.abbr(abd); got != "A u" {
		t.Errorf("abbr(/a/b/d) = %q, want %q", got, "A u")
	}
	if got := p.abbr(ax); got != "" {
		t.Errorf("abbr(/a/x) = %q, want empty", got)
	}

	// Every ancestor contributes, at any depth.
	p.set(filepath.FromSlash("/a"), permMkdir, true)
	if bits := p.inherited(abc); bits&(permUpload|permMkdir) != permUpload|permMkdir {
		t.Errorf("inherited(/a/b/c) = %v, want upload from /a/b and mkdir from /a", bits)
	}
	if bits := p.inherited(ax); bits&permUpload != 0 {
		t.Error("/a/x inherited upload from /a/b")
	}

	// An explicit bit survives being shadowed by an ancestor.
	p.set(abc, permUpload, true)
	p.set(ab, permUpload, false)
	p.set(filepath.FromSlash("/a"), permUpload, false)
	if p.get(abc)&permUpload == 0 {
		t.Error("explicit grant on /a/b/c was lost when ancestors were cleared")
	}
}

func TestDirsWithAndRoundTrip(t *testing.T) {
	p := newDirPerms()
	p.set(filepath.FromSlash("/a/b"), permUpload, true)
	p.set(filepath.FromSlash("/a/b"), permMkdir, true)
	p.set(filepath.FromSlash("/a/c"), permUpload, true)

	want := []string{filepath.FromSlash("/a/b"), filepath.FromSlash("/a/c")}
	if got := p.dirsWith(permUpload); !reflect.DeepEqual(got, want) {
		t.Errorf("dirsWith(upload) = %v, want %v", got, want)
	}
	if got := p.dirsWith(permDelete); got != nil {
		t.Errorf("dirsWith(delete) = %v, want nil", got)
	}

	back := dirPermsFromJSON(p.toJSON())
	if !reflect.DeepEqual(back.m, p.m) {
		t.Errorf("round trip = %v, want %v", back.m, p.m)
	}
}

func TestPruneDropsOutsideRootAndMissing(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "keep")
	if err := os.Mkdir(keep, 0o755); err != nil {
		t.Fatal(err)
	}

	p := newDirPerms()
	p.set(keep, permUpload, true)
	p.set(filepath.Join(root, "gone"), permUpload, true)
	p.set(filepath.FromSlash("/elsewhere"), permUpload, true)

	p.prune(root)
	if _, ok := p.m[keep]; !ok {
		t.Error("prune dropped an existing directory under root")
	}
	if len(p.m) != 1 {
		t.Errorf("prune left %v, want only %q", p.m, keep)
	}

	p.prune("")
	if len(p.m) != 0 {
		t.Errorf("prune with empty root left %v", p.m)
	}
}

// nativePath is what keeps the Root entry spelled the same way as the Directory
// tree, which builds its rows from filepath — including the empty case, where
// filepath.Clean would turn a blank entry into ".".
func TestNativePath(t *testing.T) {
	cases := [][2]string{
		{"", ""},
		{"/a/b", "/a/b"},
		{"/a/b/", "/a/b"},
		{"/a//b", "/a/b"},
		{"/a/./b", "/a/b"},
	}
	for _, c := range cases {
		in, want := c[0], filepath.FromSlash(c[1])
		if got := nativePath(in); got != want {
			t.Errorf("nativePath(%q) = %q, want %q", in, got, want)
		}
		// Whatever separator arrives, the result must be the native spelling.
		if got := nativePath(filepath.FromSlash(in)); got != want {
			t.Errorf("nativePath(%q) = %q, want %q", filepath.FromSlash(in), got, want)
		}
	}
}
