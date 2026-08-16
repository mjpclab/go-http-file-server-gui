package main

import (
	"reflect"
	"testing"
)

func TestParseMultiValues(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{",", nil},
		{" , , ", nil},
		{"8080", []string{"8080"}},
		{"8080 8081", []string{"8080", "8081"}},
		{"8080,8081", []string{"8080", "8081"}},
		{"8080, 8081", []string{"8080", "8081"}},
		{"  8080 ,,  8081  ", []string{"8080", "8081"}},
		{":80 127.0.0.1:8080,[::1]:8081", []string{":80", "127.0.0.1:8080", "[::1]:8081"}},
		// Only " " and "," separate. A tab is part of the value, matching what
		// the entry shows: nothing in the form makes whitespace visible, so a
		// pasted tab silently splitting a wildcard would be unexplainable.
		{"a\tb", []string{"a\tb"}},

		// A value opening with a double quote runs to the closing quote, so the
		// separators inside it are literal.
		{`"My Docs*"`, []string{"My Docs*"}},
		{`"My Docs*" *.tmp`, []string{"My Docs*", "*.tmp"}},
		{`"a b" "c,d"`, []string{"a b", "c,d"}},
		{`"cache-control:public, max-age=0"`, []string{"cache-control:public, max-age=0"}},
		{`"a,b",c`, []string{"a,b", "c"}},

		// Only the opening position is special. A quote anywhere else is an
		// ordinary character, which is what keeps a header value that is itself
		// quoted (ETag, Content-Disposition) typable with no escape syntax.
		{`etag:"abc123"`, []string{`etag:"abc123"`}},
		{`a"b`, []string{`a"b`}},
		{`cache-control:"public, max-age=0"`, []string{`cache-control:"public`, `max-age=0"`}},

		// Degenerate quoting stays lossless rather than dropping input: text
		// after the closing quote joins the same value, and a quote left open
		// runs to the end.
		{`"a"b`, []string{"ab"}},
		{`"a b`, []string{"a b"}},
		{`""`, nil},
		{`"" x`, []string{"x"}},
	}
	for _, c := range cases {
		if got := parseMultiValues(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseMultiValues(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}
