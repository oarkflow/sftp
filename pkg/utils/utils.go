package utils

import (
	"path/filepath"
)

func AbsPath(path string) string {
	if !filepath.IsAbs(path) {
		b, err := filepath.Abs(path)
		if err == nil {
			path = b
		}
	}
	return path
}
