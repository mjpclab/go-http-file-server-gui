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
	}
	for _, c := range cases {
		if got := parseMultiValues(c.in); !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseMultiValues(%q) = %#v, want %#v", c.in, got, c.want)
		}
	}
}
