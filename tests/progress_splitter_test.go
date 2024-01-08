package tests

import (
	"testing"

	"github.com/madconst/youtube-dl-bot/utils"
)

func join(lines []string, remainder string) string {
	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result + remainder
}

func makeCheck(t *testing.T) func(remainder string, chunk string, expected string) {
	return func(remainder string, chunk string, expected string) {
		got := join(utils.ProcessConsoleOutput(remainder, chunk))
		if got != expected {
			t.Errorf("Got '%s', expected '%s'", got, expected)
		}
	}
}

func TestProgressSplitter(t *testing.T) {
	check := makeCheck(t)
	check("", "", "")                 // "", "" -> [], "" -> ""
	check("", "\rabc", "abc")         // "", "\rabc" -> [], "abc" -> "abc"
	check("", "\rabc\rdef", "def")    // "", "\rabc\rdef" -> [], "def" -> "def"
	check("abc", "\rdef", "def")      // "abc", "\rdef" -> [], "def" -> "def"
	check("abc", "\ndef", "abc\ndef") // "abc", "\ndef" -> ["abc"], "def" -> "abc\ndef"
	check("abc", "\ndef\r\n", "abc\n\n")
}
