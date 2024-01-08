package utils

import (
	"fmt"
	"strings"
)

func maskSpecialChars(s string) string {
	s = strings.Replace(s, "\r", "<r>", -1)
	return strings.Replace(s, "\n", "<n>", -1)
}

func ProcessConsoleOutput(remainder string, chunk string) ([]string, string) {
	fmt.Printf("Remainder: '%s', chunk: '%s'\n", maskSpecialChars(remainder), maskSpecialChars(chunk))
	lines := []string{}
	start := 0
	for i := 0; i < len(chunk); i++ {
		if chunk[i] == '\r' {
			remainder = ""
			start = i + 1
			continue
		}
		if chunk[i] == '\n' {
			lines = append(lines, remainder+chunk[start:i])
			remainder = ""
			start = i + 1
			continue
		}
	}
	fmt.Printf("Lines: %v, remainder: '%v', start: %v\n", lines, maskSpecialChars(remainder), start)
	return lines, chunk[start:]
}
