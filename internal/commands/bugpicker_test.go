package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/discord"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

func values(choices []discord.AutocompleteChoice) []string {
	var out []string
	for _, c := range choices {
		out = append(out, c.(discord.AutocompleteChoiceString).Value)
	}

	return out
}

func TestBugChoices(t *testing.T) {
	items := []bugtracker.Item{
		{ID: "a", Number: 1, Name: "login button broken"},
		{ID: "b", Number: 2, Name: "crash on start", Complete: true},
		{ID: "c", Number: 3, Name: "login page slow [screenshots: https://discord.com/channels/1/2/3]"},
	}

	tests := []struct {
		query string
		want  string
	}{
		{"", "c a b"},    // open bugs newest first, fixed last
		{"login", "c a"}, // text match
		{"2", "b"},       // list number match
		{"nothing", ""},  // no match
		{"SLOW", "c"},    // case-insensitive
	}

	for _, tt := range tests {
		got := strings.Join(values(bugChoices(items, tt.query)), " ")
		if got != tt.want {
			t.Errorf("query %q: got %q, want %q", tt.query, got, tt.want)
		}
	}
}

func TestBugChoicesLimit(t *testing.T) {
	var items []bugtracker.Item
	for i := 1; i <= 40; i++ {
		items = append(items, bugtracker.Item{ID: fmt.Sprint(i), Number: i, Name: "bug"})
	}

	if got := len(bugChoices(items, "")); got != maxChoices {
		t.Errorf("got %d choices, want %d", got, maxChoices)
	}
}

func TestChoiceLabel(t *testing.T) {
	item := bugtracker.Item{Number: 3, Name: "login page slow (Added by Pat) [screenshots: https://discord.com/channels/1/2/3]"}
	if got, want := choiceLabel(item), "3) login page slow (Added by Pat)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	long := bugtracker.Item{Number: 1, Name: strings.Repeat("x", 300)}
	if got := len([]rune(choiceLabel(long))); got > maxChoiceChars {
		t.Errorf("label is %d runes, limit %d", got, maxChoiceChars)
	}
}
