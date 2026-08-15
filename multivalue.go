package main

import "strings"

// parseMultiValues splits one form field holding several values into one string
// per value. Spaces and commas both separate, in any combination and any
// number, so "8080 8081", "8080,8081" and "8080, 8081" all yield two values.
//
// An empty field yields nil rather than [""], which matters at the ghfs
// boundary: every field this feeds is a []string where the empty slice means
// "unset" and gets a default, while a slice holding one empty string is a real
// value — an empty Listens becomes :80 or :443, but [""] is an address ghfs
// then fails to bind.
func parseMultiValues(s string) []string {
	values := strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == ','
	})
	if len(values) == 0 {
		return nil
	}
	return values
}
