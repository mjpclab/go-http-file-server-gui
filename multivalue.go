package main

import "strings"

// parseMultiValues splits one form field holding several values into one string
// per value. Spaces and commas both separate, in any combination and any
// number, so "8080 8081", "8080,8081" and "8080, 8081" all yield two values.
//
// A value that *opens* with a double quote runs to the closing quote, so the
// separators inside it are literal: "My Docs*" is one wildcard, and a header
// entry written "cache-control:public, max-age=0" survives its comma. Only the
// opening position is special — a quote anywhere else is an ordinary character.
// That asymmetry is the whole escape story: there is no way to write a literal
// quote at the start of a value, and in exchange a header value that is itself
// quoted, etag:"abc123", needs no escape syntax at all and round-trips as typed.
//
// Degenerate quoting stays lossless rather than dropping what was typed: text
// following the closing quote joins the same value, and a quote left open runs
// to the end of the field.
//
// An empty field yields nil rather than [""], which matters at the ghfs
// boundary: every field this feeds is a []string where the empty slice means
// "unset" and gets a default, while a slice holding one empty string is a real
// value — an empty Listens becomes :80 or :443, but [""] is an address ghfs
// then fails to bind.
func parseMultiValues(s string) []string {
	isSep := func(b byte) bool { return b == ' ' || b == ',' }

	var values []string
	for i := 0; i < len(s); {
		if isSep(s[i]) {
			i++
			continue
		}

		var value strings.Builder
		if s[i] == '"' {
			i++
			for i < len(s) && s[i] != '"' {
				value.WriteByte(s[i])
				i++
			}
			if i < len(s) {
				i++ // the closing quote itself
			}
		}
		for i < len(s) && !isSep(s[i]) {
			value.WriteByte(s[i])
			i++
		}

		if value.Len() > 0 {
			values = append(values, value.String())
		}
	}
	return values
}
