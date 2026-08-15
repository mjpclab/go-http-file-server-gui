package main

import (
	"regexp"
	"strings"

	"mjpclab.dev/ghfs/src/util"
)

// newHideFilter compiles the Hide wildcards into the regexp ghfs matches names
// against, so the Directory tab can mark the rows the server will leave out of
// its listings. It mirrors serverHandler.wildcardToRegexp, which is unexported,
// and must stay in step with it — the shared step is util.WildcardToStrRegexp,
// which anchors each pattern at both ends and carries the build tags making the
// match case-insensitive on windows/darwin, so the tree agrees with the server
// per platform without this file knowing about any of that.
//
// nil means "nothing is filtered", the same signal ghfs uses for an empty Hide.
// A pattern that will not compile also yields nil rather than an error: Start
// surfaces the real complaint from ghfs, and until then a tree that marks
// nothing is better than one that marks the wrong rows.
//
// Note what this deliberately does not model: filtering only removes a name
// from its parent's listing. The directory stays reachable by URL, still lists
// its own contents, and any permission granted on it still applies — which is
// why the rows it marks stay interactive.
func newHideFilter(wildcards []string) *regexp.Regexp {
	exps := make([]string, 0, len(wildcards))
	for _, wildcard := range wildcards {
		if len(wildcard) == 0 {
			continue
		}
		exps = append(exps, util.WildcardToStrRegexp(wildcard))
	}
	if len(exps) == 0 {
		return nil
	}

	re, err := regexp.Compile(strings.Join(exps, "|"))
	if err != nil {
		return nil
	}
	return re
}
