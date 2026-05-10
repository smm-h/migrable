package engine

import (
	"fmt"
	"strings"
)

// SplitFileKey splits a dot-separated path into a file key and the inner path
// within that file. For single-file projects (one key), the full path is returned
// unchanged as the inner path. For multi-file projects, the first unescaped dot
// segment must match one of the file keys.
func SplitFileKey(path string, fileKeys []string) (fileKey string, innerPath string, err error) {
	if len(fileKeys) == 0 {
		return "", "", fmt.Errorf("no file keys configured")
	}

	if len(fileKeys) == 1 {
		return fileKeys[0], path, nil
	}

	firstSeg := firstSegment(path)

	for _, k := range fileKeys {
		if firstSeg == k {
			rest := path[len(firstSeg):]
			if rest == "" {
				return "", "", fmt.Errorf("path %q is just a file key with no inner path", path)
			}
			// rest starts with ".", strip it
			return k, rest[1:], nil
		}
	}

	return "", "", fmt.Errorf("path %q does not start with a known file key (valid keys: %s)",
		path, strings.Join(fileKeys, ", "))
}

// firstSegment returns the portion of path before the first unescaped dot.
// Escaped dots (\.) are not treated as separators.
func firstSegment(path string) string {
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+1 < len(path) {
			i++ // skip the escaped character
			continue
		}
		if path[i] == '.' {
			return path[:i]
		}
	}
	return path
}
