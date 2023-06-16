package main

import "strings"

func split(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == '\r' || r == '\n' })
	for i, f := range fields {
		fields[i] = strings.TrimSpace(f)
	}
	return fields
}

func trim(s string, maxlen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxlen {
		s = s[:maxlen]
	}
	return s
}
