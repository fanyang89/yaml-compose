package compose

import (
	"fmt"
	"regexp"
	"strings"
)

// applyListFilter returns a filtered subset of input, keeping only items
// whose match candidate satisfies the include/exclude regex rules.
func applyListFilter(input []any, filter layerListFilter) ([]any, error) {
	out := make([]any, 0, len(input))
	for i, item := range input {
		candidate, err := getFilterCandidate(item, filter.matchPath)
		if err != nil {
			return nil, fmt.Errorf("invalid list item at index %d: %w", i, err)
		}

		if !shouldKeepCandidate(candidate, filter.include, filter.exclude, filter.includeMode) {
			continue
		}
		out = append(out, item)
	}

	return out, nil
}

// applyListExtract extracts a string field from each object in input,
// returning a flat string list filtered by the extract rules.
func applyListExtract(input []any, extract layerListExtract) ([]any, error) {
	out := make([]any, 0, len(input))
	for i, item := range input {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid list item at index %d: expected object list item", i)
		}

		v, ok := getValueAtPath(m, extract.extractPath)
		if !ok {
			return nil, fmt.Errorf("invalid list item at index %d: extract_path %q not found", i, normalizePath(extract.extractPath))
		}

		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid list item at index %d: extract_path %q must resolve to string", i, normalizePath(extract.extractPath))
		}

		if !shouldKeepCandidate(s, extract.include, extract.exclude, extract.includeMode) {
			continue
		}

		out = append(out, s)
	}

	return out, nil
}

// applyReplaceValues is the operator-level entry point for replace_values;
// it delegates to replaceValues and returns the replaced value plus the list
// of original strings that were changed.
func applyReplaceValues(input any, replace layerReplaceValues) (any, []string) {
	return replaceValues(input, replace.old, replace.new, replace.recursive)
}

func replaceValues(input any, old string, new string, recursive bool) (any, []string) {
	s, ok := input.(string)
	if ok {
		replaced := strings.ReplaceAll(s, old, new)
		if replaced != s {
			return replaced, []string{s}
		}
		return replaced, nil
	}

	m, ok := input.(map[string]any)
	if ok {
		out := make(map[string]any, len(m))
		originals := make([]string, 0)
		for k, v := range m {
			if recursive {
				replaced, childOriginals := replaceValues(v, old, new, true)
				out[k] = replaced
				originals = append(originals, childOriginals...)
				continue
			}
			if sv, isString := v.(string); isString {
				replaced := strings.ReplaceAll(sv, old, new)
				if replaced != sv {
					originals = append(originals, sv)
				}
				out[k] = replaced
				continue
			}
			out[k] = v
		}
		return out, originals
	}

	list, ok := input.([]any)
	if ok {
		out := make([]any, len(list))
		originals := make([]string, 0)
		for i, v := range list {
			if recursive {
				replaced, childOriginals := replaceValues(v, old, new, true)
				out[i] = replaced
				originals = append(originals, childOriginals...)
				continue
			}
			if sv, isString := v.(string); isString {
				replaced := strings.ReplaceAll(sv, old, new)
				if replaced != sv {
					originals = append(originals, sv)
				}
				out[i] = replaced
				continue
			}
			out[i] = v
		}
		return out, originals
	}

	return input, nil
}

func getFilterCandidate(item any, matchPath []string) (string, error) {
	if len(matchPath) == 0 {
		if _, isObject := item.(map[string]any); isObject {
			return "", fmt.Errorf("object list requires transform.list_filter.match_path")
		}
		s, ok := item.(string)
		if !ok {
			return "", fmt.Errorf("expected string list item")
		}
		return s, nil
	}

	m, ok := item.(map[string]any)
	if !ok {
		return "", fmt.Errorf("expected object list item for match_path %q", normalizePath(matchPath))
	}

	v, ok := getValueAtPath(m, matchPath)
	if !ok {
		return "", fmt.Errorf("match_path %q not found", normalizePath(matchPath))
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("match_path %q must resolve to string", normalizePath(matchPath))
	}

	return s, nil
}

func matchesInclude(candidate string, include []*regexp.Regexp, mode includeMode) bool {
	if len(include) == 0 {
		return true
	}

	if mode == includeModeAll {
		for _, re := range include {
			if !re.MatchString(candidate) {
				return false
			}
		}
		return true
	}

	return matchesAny(candidate, include)
}

func matchesAny(candidate string, regex []*regexp.Regexp) bool {
	for _, re := range regex {
		if re.MatchString(candidate) {
			return true
		}
	}
	return false
}

func shouldKeepCandidate(candidate string, include []*regexp.Regexp, exclude []*regexp.Regexp, mode includeMode) bool {
	if !matchesInclude(candidate, include, mode) {
		return false
	}
	if matchesAny(candidate, exclude) {
		return false
	}
	return true
}
