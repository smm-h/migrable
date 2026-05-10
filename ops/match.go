package ops

import (
	"fmt"
	"regexp"
)

type MatchMode string

const (
	MatchSubset    MatchMode = "subset"
	MatchExact     MatchMode = "exact"
	MatchAll       MatchMode = "all"
	MatchIndex     MatchMode = "index"
	MatchHasKey    MatchMode = "has_key"
	MatchNotHasKey MatchMode = "not_has_key"
	MatchRegex     MatchMode = "regex"
)

// Matches checks whether item matches the where clause according to the given
// mode. arrayLen is the total array length, needed for negative index
// normalization. index is the current element's position in the array.
func Matches(item map[string]any, where any, mode MatchMode, index int, arrayLen int) bool {
	switch mode {
	case MatchAll:
		return true

	case MatchIndex:
		return matchIndex(index, where, arrayLen)

	case MatchHasKey:
		key, ok := where.(string)
		if !ok {
			return false
		}
		_, exists := item[key]
		return exists

	case MatchNotHasKey:
		key, ok := where.(string)
		if !ok {
			return false
		}
		_, exists := item[key]
		return !exists

	case MatchSubset, "":
		whereMap, ok := where.(map[string]any)
		if !ok {
			return false
		}
		return matchSubset(item, whereMap)

	case MatchExact:
		whereMap, ok := where.(map[string]any)
		if !ok {
			return false
		}
		return matchExact(item, whereMap)

	case MatchRegex:
		whereMap, ok := where.(map[string]any)
		if !ok {
			return false
		}
		return matchRegex(item, whereMap)

	default:
		return false
	}
}

func matchIndex(index int, where any, arrayLen int) bool {
	var target int
	switch v := where.(type) {
	case int:
		target = v
	case int64:
		target = int(v)
	case float64:
		target = int(v)
	default:
		return false
	}

	if target < 0 {
		target = arrayLen + target
	}
	if target < 0 || target >= arrayLen {
		return false
	}
	return index == target
}

func matchSubset(item, where map[string]any) bool {
	for k, wv := range where {
		iv, exists := item[k]
		if !exists {
			return false
		}
		if fmt.Sprintf("%v", iv) != fmt.Sprintf("%v", wv) {
			return false
		}
	}
	return true
}

func matchExact(item, where map[string]any) bool {
	if len(item) != len(where) {
		return false
	}
	return matchSubset(item, where)
}

func matchRegex(item, where map[string]any) bool {
	for k, wv := range where {
		wStr, isStr := wv.(string)
		if isStr {
			iv, exists := item[k]
			if !exists {
				return false
			}
			ivStr, ivIsStr := iv.(string)
			if !ivIsStr {
				return false
			}
			pattern := anchorPattern(wStr)
			matched, err := regexp.MatchString(pattern, ivStr)
			if err != nil || !matched {
				return false
			}
		} else {
			iv, exists := item[k]
			if !exists {
				return false
			}
			if fmt.Sprintf("%v", iv) != fmt.Sprintf("%v", wv) {
				return false
			}
		}
	}
	return true
}

func anchorPattern(pattern string) string {
	hasStart := len(pattern) > 0 && pattern[0] == '^'
	hasEnd := len(pattern) > 0 && pattern[len(pattern)-1] == '$'
	if !hasStart {
		pattern = "^" + pattern
	}
	if !hasEnd {
		pattern = pattern + "$"
	}
	return pattern
}
