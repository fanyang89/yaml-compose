package compose

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errPathSelectorNoMatch = errors.New("selector matched no array item")

// getValueAtPath traverses root following the given path segments and returns
// the value found, or (nil, false) if any segment is not reachable.
func getValueAtPath(root any, path []string) (any, bool) {
	if len(path) == 0 {
		return root, true
	}

	cur := root
	for _, segment := range path {
		switch typed := cur.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			cur = next
		case []any:
			if index, ok := parsePathIndex(segment); ok {
				if index < 0 || index >= len(typed) {
					return nil, false
				}
				cur = typed[index]
				continue
			}

			selectorKey, selectorValue, ok := parsePathSelector(segment)
			if !ok {
				return nil, false
			}
			index, ok := findArrayObjectBySelector(typed, selectorKey, selectorValue)
			if !ok {
				return nil, false
			}
			cur = typed[index]
		default:
			return nil, false
		}
	}

	return cur, true
}

// setMapValueAtPath writes value into root at the given path, creating
// intermediate map nodes as needed.  Returns an error if the path is
// unwritable (e.g. out-of-range index, ambiguous selector).
func setMapValueAtPath(root map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("path cannot be empty")
	}

	var cur any = root
	for i := 0; i < len(path)-1; i++ {
		segment := path[i]
		switch typed := cur.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				if _, isIndex := parsePathIndex(path[i+1]); isIndex {
					return fmt.Errorf("target path %q is not writable: segment %q must be an array", normalizePath(path), normalizePath(path[:i+1]))
				}
				child := map[string]any{}
				typed[segment] = child
				cur = child
				continue
			}
			cur = next
		case []any:
			if index, ok := parsePathIndex(segment); ok {
				if index < 0 || index >= len(typed) {
					return fmt.Errorf("target path %q is not writable: index %d out of range at segment %q", normalizePath(path), index, normalizePath(path[:i+1]))
				}
				cur = typed[index]
				continue
			}

			selectorKey, selectorValue, ok := parsePathSelector(segment)
			if !ok {
				return fmt.Errorf("target path %q is not writable: segment %q must be an array index or selector", normalizePath(path), normalizePath(path[:i+1]))
			}

			index, err := findUniqueArrayObjectBySelector(typed, selectorKey, selectorValue)
			if err != nil {
				return fmt.Errorf("target path %q is not writable: %w", normalizePath(path), err)
			}
			cur = typed[index]
		default:
			return fmt.Errorf("target path %q is not writable: segment %q is not an object or array", normalizePath(path), normalizePath(path[:i+1]))
		}
	}

	lastSegment := path[len(path)-1]
	switch typed := cur.(type) {
	case map[string]any:
		typed[lastSegment] = value
		return nil
	case []any:
		if index, ok := parsePathIndex(lastSegment); ok {
			if index < 0 || index >= len(typed) {
				return fmt.Errorf("target path %q is not writable: final index %d out of range", normalizePath(path), index)
			}
			typed[index] = value
			return nil
		}

		selectorKey, selectorValue, ok := parsePathSelector(lastSegment)
		if !ok {
			return fmt.Errorf("target path %q is not writable: final segment %q must be an array index or selector", normalizePath(path), normalizePath(path))
		}
		index, err := findUniqueArrayObjectBySelector(typed, selectorKey, selectorValue)
		if err != nil {
			return fmt.Errorf("target path %q is not writable: %w", normalizePath(path), err)
		}
		typed[index] = value
		return nil
	default:
		return fmt.Errorf("target path %q is not writable: parent is not an object or array", normalizePath(path))
	}
}

func parsePathIndex(segment string) (int, bool) {
	index, err := strconv.Atoi(segment)
	if err != nil {
		return 0, false
	}
	return index, true
}

func parsePathSelector(segment string) (string, string, bool) {
	key, value, ok := strings.Cut(segment, "=")
	if !ok {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", "", false
	}

	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		if len(value) < 2 {
			return "", "", false
		}

		if strings.HasPrefix(value, "'") {
			inner := value[1 : len(value)-1]
			inner = strings.ReplaceAll(inner, `\'`, `'`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			return key, inner, true
		}

		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", "", false
		}
		return key, unquoted, true
	}

	return key, value, true
}

func findArrayObjectBySelector(items []any, key string, expected string) (int, bool) {
	index, err := findUniqueArrayObjectBySelector(items, key, expected)
	if err != nil {
		return 0, false
	}
	return index, true
}

func findUniqueArrayObjectBySelector(items []any, key string, expected string) (int, error) {
	matches := make([]int, 0)
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return 0, fmt.Errorf("selector [%s=%s] requires object array items, got %T at index %d", key, expected, item, i)
		}
		raw, ok := m[key]
		if !ok {
			continue
		}
		actual, ok := raw.(string)
		if !ok {
			return 0, fmt.Errorf("selector [%s=%s] requires string field %q, got %T at index %d", key, expected, key, raw, i)
		}
		if actual == expected {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return 0, fmt.Errorf("%w: selector [%s=%s]", errPathSelectorNoMatch, key, expected)
	}
	if len(matches) > 1 {
		return 0, fmt.Errorf("selector [%s=%s] matched multiple array items", key, expected)
	}

	return matches[0], nil
}

// normalizeDotPath parses a dot-notation path string and returns the
// canonical normalized form used as map key in merge strategy paths.
func normalizeDotPath(path string) (string, error) {
	parts, err := splitDotPath(path)
	if err != nil {
		return "", err
	}
	return normalizePath(parts), nil
}

// splitDotPath splits a dot-separated path string into individual segments,
// honouring backslash escapes and bracket notation for array indices and
// selectors.
func splitDotPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	parts := make([]string, 0)
	var segment strings.Builder
	escaped := false
	for i := 0; i < len(path); i++ {
		ch := path[i]
		if escaped {
			segment.WriteByte(ch)
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '.':
			if segment.Len() == 0 {
				return nil, fmt.Errorf("empty segment")
			}
			parts = append(parts, segment.String())
			segment.Reset()
		default:
			segment.WriteByte(ch)
		}
	}

	if escaped {
		return nil, fmt.Errorf("path cannot end with an escape character")
	}
	if segment.Len() == 0 {
		return nil, fmt.Errorf("empty segment")
	}
	parts = append(parts, segment.String())

	expandedParts, err := expandBracketPathSegments(parts)
	if err != nil {
		return nil, err
	}

	return expandedParts, nil
}

func expandBracketPathSegments(parts []string) ([]string, error) {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		expanded, err := expandBracketPathSegment(part)
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func expandBracketPathSegment(segment string) ([]string, error) {
	out := make([]string, 0, 1)
	var token strings.Builder
	afterIndex := false

	for i := 0; i < len(segment); {
		ch := segment[i]
		if ch != '[' {
			if afterIndex {
				return nil, fmt.Errorf("invalid bracket path segment %q", segment)
			}
			token.WriteByte(ch)
			i++
			continue
		}

		if token.Len() > 0 {
			out = append(out, token.String())
			token.Reset()
		}

		end := i + 1
		for end < len(segment) && segment[end] != ']' {
			end++
		}
		if end >= len(segment) {
			return nil, fmt.Errorf("invalid bracket path segment %q", segment)
		}

		indexRaw := segment[i+1 : end]
		if indexRaw == "" {
			return nil, fmt.Errorf("invalid bracket path segment %q", segment)
		}
		if _, err := strconv.Atoi(indexRaw); err != nil {
			if _, _, ok := parsePathSelector(indexRaw); !ok {
				return nil, fmt.Errorf("invalid bracket path segment %q", segment)
			}
		}

		out = append(out, indexRaw)
		afterIndex = true
		i = end + 1
	}

	if token.Len() > 0 {
		out = append(out, token.String())
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("empty segment")
	}

	return out, nil
}

// normalizePath converts a slice of path segments back to the canonical
// dot-separated string, escaping dots and backslashes in segment names.
func normalizePath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, escapeDotPathSegment(part))
	}
	return strings.Join(encoded, ".")
}

func escapeDotPathSegment(part string) string {
	part = strings.ReplaceAll(part, `\\`, `\\\\`)
	part = strings.ReplaceAll(part, ".", `\\.`)
	return part
}
