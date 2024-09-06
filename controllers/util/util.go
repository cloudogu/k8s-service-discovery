package util

import "strings"

func ContainsChars(s string) bool {
	return len(strings.TrimSpace(s)) != 0
}
