package api

import (
	"errors"
	"path"
	"strings"
)

var ErrBadPath = errors.New("invalid path")

// CleanPath normalises a client-supplied remote path against root and rejects
// anything that escapes it. Returns an absolute, slash-separated path.
func CleanPath(root, p string) (string, error) {
	if strings.ContainsRune(p, 0) {
		return "", ErrBadPath
	}
	root = path.Clean("/" + strings.TrimPrefix(root, "/"))
	if p == "" {
		return root, nil
	}
	if !strings.HasPrefix(p, "/") {
		p = path.Join(root, p)
	}
	cleaned := path.Clean(p)
	if cleaned != root && !strings.HasPrefix(cleaned, strings.TrimSuffix(root, "/")+"/") {
		return "", ErrBadPath
	}
	return cleaned, nil
}
