package main

import (
	"runtime"
	"testing"
)

func TestNewHideFilterNilCases(t *testing.T) {
	for _, in := range [][]string{nil, {}, {""}, {"", ""}} {
		if re := newHideFilter(in); re != nil {
			t.Errorf("newHideFilter(%#v) = %v, want nil", in, re)
		}
	}
}

func TestHideFilterMatches(t *testing.T) {
	cases := []struct {
		wildcards []string
		name      string
		want      bool
	}{
		// Anchored at both ends: a wildcard matches a whole name, never a part.
		{[]string{"build"}, "build", true},
		{[]string{"build"}, "rebuild", false},
		{[]string{"build"}, "builds", false},
		// "*" and "?" are the only metacharacters.
		{[]string{"*.log"}, "server.log", true},
		{[]string{"*.log"}, "log", false},
		{[]string{".*"}, ".git", true},
		{[]string{".*"}, "git", false},
		{[]string{"v?"}, "v1", true},
		{[]string{"v?"}, "v10", false},
		// Regexp metacharacters are literals.
		{[]string{"a+b"}, "a+b", true},
		{[]string{"a+b"}, "aab", false},
		{[]string{"x(y)"}, "x(y)", true},
		// Several patterns alternate.
		{[]string{"*.syso", "*.png"}, "Icon.png", true},
		{[]string{"*.syso", "*.png"}, "rc.syso", true},
		{[]string{"*.syso", "*.png"}, "Icon.icns", false},
		// An empty pattern among real ones is dropped, not turned into "matches
		// everything" — ghfs skips it the same way.
		{[]string{"", "*.log"}, "a.log", true},
		{[]string{"", "*.log"}, "a.txt", false},
	}
	for _, c := range cases {
		re := newHideFilter(c.wildcards)
		if re == nil {
			t.Errorf("newHideFilter(%#v) = nil, want a filter", c.wildcards)
			continue
		}
		if got := re.MatchString(c.name); got != c.want {
			t.Errorf("newHideFilter(%#v).MatchString(%q) = %v, want %v",
				c.wildcards, c.name, got, c.want)
		}
	}
}

// Case sensitivity comes from util.WildcardToStrRegexp's build tags, so that the
// tree marks exactly the rows the server will filter on this platform.
func TestHideFilterCaseFollowsPlatform(t *testing.T) {
	got := newHideFilter([]string{"icon.png"}).MatchString("Icon.PNG")
	want := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	if got != want {
		t.Errorf("case-insensitive match on %s = %v, want %v", runtime.GOOS, got, want)
	}
}
