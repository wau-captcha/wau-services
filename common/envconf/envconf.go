// Package envconf fills service config from the environment.
//
// config.yaml keeps no environment-specific values: secrets, addresses and URLs are
// written as ${NAME} and read from the environment variable NAME, which a service's
// .env provides at startup. Placeholders take no inline default, so a missing value
// stops the service instead of silently falling back to one.
package envconf

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

var placeholder = regexp.MustCompile(`\$\{([^}]*)\}`)

// Resolve replaces every ${NAME} in the config values with the variable NAME, which
// must be set and not blank. It is a Kratos config.Resolver.
func Resolve(values map[string]any) error {
	var missing, defaulted []string
	expand := func(s string) string {
		return placeholder.ReplaceAllStringFunc(s, func(match string) string {
			name := strings.TrimSpace(match[2 : len(match)-1])
			if strings.Contains(name, ":") {
				defaulted = append(defaulted, match)
				return match
			}
			value, ok := os.LookupEnv(name)
			if !ok || strings.TrimSpace(value) == "" {
				missing = append(missing, name)
				return match
			}
			return value
		})
	}
	walk(values, expand)

	var errs []error
	if len(defaulted) > 0 {
		errs = append(errs, fmt.Errorf("config placeholders must not carry a default, set the value in .env instead: %s",
			strings.Join(defaulted, ", ")))
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		errs = append(errs, fmt.Errorf("environment variables not set, add them to .env: %s",
			strings.Join(slices.Compact(missing), ", ")))
	}
	return errors.Join(errs...)
}

func walk(value any, expand func(string) string) any {
	switch v := value.(type) {
	case string:
		return expand(v)
	case map[string]any:
		for key, item := range v {
			v[key] = walk(item, expand)
		}
	case []any:
		for i, item := range v {
			v[i] = walk(item, expand)
		}
	}
	return value
}

// List splits a comma-separated value, the way lists are written in .env. Blank items are dropped.
func List(value string) []string {
	var items []string
	for item := range strings.SplitSeq(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
}
