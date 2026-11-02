package blacklist

import (
	"fmt"
	"regexp"
	"strings"
)

type pathMatcher struct{ expression *regexp.Regexp }

func (m pathMatcher) Match(ctx Context) bool {
	return ctx.Repository != "" && m.expression.MatchString(strings.Trim(ctx.Repository, "/"))
}

func normalizePathPattern(pattern string) (string, error) {
	original := pattern
	pattern = strings.Trim(strings.TrimSpace(pattern), "/")
	if pattern == "" || strings.Contains(pattern, "//") || strings.ContainsAny(pattern, " :@?[]\\\t\r\n") {
		return "", fmt.Errorf("黑名单匹配表达式无效: %q", original)
	}
	return pattern, nil
}

func compilePathPattern(pattern string) (Matcher, error) {
	expression := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
	compiled, err := regexp.Compile(expression)
	if err != nil {
		return nil, err
	}
	return pathMatcher{compiled}, nil
}

// compileRepository validates and compiles an exact or wildcard image path.
func compileRepository(pattern string) (Matcher, error) {
	pattern, err := normalizePathPattern(pattern)
	if err != nil {
		return nil, err
	}
	return compilePathPattern(pattern)
}

// compileNamespace validates a namespace and matches all repositories below it.
func compileNamespace(pattern string) (Matcher, error) {
	pattern, err := normalizePathPattern(pattern)
	if err != nil {
		return nil, err
	}
	return compilePathPattern(pattern + "/*")
}
