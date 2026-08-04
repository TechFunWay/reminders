// Package utils holds small, dependency-light helpers shared across the
// framework and the apps built on top of it.
package utils

import "strconv"

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// NormalizePage clamps page/pageSize into sane bounds. page defaults to 1,
// pageSize to DefaultPageSize and is capped at MaxPageSize.
func NormalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// Offset returns the SQL OFFSET for a (page, pageSize) pair.
func Offset(page, pageSize int) int {
	return (page - 1) * pageSize
}

// Atoi parses s, returning def when s is empty or not a number. Handy for
// optional query parameters.
func Atoi(s string, def int) int {
	if s == "" {
		return def
	}
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	return def
}
