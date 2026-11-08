package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

func TestFormatBugList(t *testing.T) {
	items := []bugtracker.Item{
		{Number: 1, Name: "first"},
		{Number: 2, Name: "second", Complete: true},
		{Number: 3, Name: "third"},
	}

	t.Run("all items", func(t *testing.T) {
		got := formatBugList(items, false)

		want := "**1) first**\n\n~~2) second~~\n\n**3) third**\n\n"
		if len(got) != 1 || got[0] != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("incomplete only keeps original numbers", func(t *testing.T) {
		got := formatBugList(items, true)

		want := "**1) first**\n\n**3) third**\n\n"
		if len(got) != 1 || got[0] != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("message links are masked", func(t *testing.T) {
		link := "https://discord.com/channels/1/2/3"
		got := formatBugList([]bugtracker.Item{{Number: 1, Name: "bug [" + link + "]"}}, false)

		if want := "[link](" + link + ")"; !strings.Contains(got[0], want) {
			t.Errorf("got %q, want it to contain %q", got[0], want)
		}
	})
}

func TestFormatBugListEmpty(t *testing.T) {
	tests := []struct {
		name           string
		items          []bugtracker.Item
		incompleteOnly bool
	}{
		{"no items", nil, false},
		{"all filtered out", []bugtracker.Item{{Name: "done", Complete: true}}, true},
	}

	for _, tt := range tests {
		got := formatBugList(tt.items, tt.incompleteOnly)
		if len(got) != 1 || got[0] != "No bugs found!" {
			t.Errorf("%s: got %q", tt.name, got)
		}
	}
}

func TestFormatBugListOversizedItem(t *testing.T) {
	items := []bugtracker.Item{{Number: 1, Name: strings.Repeat("x", maxMessageLength+100)}}

	got := formatBugList(items, false)
	if len(got) != 1 || !strings.HasPrefix(got[0], "**1) xxx") {
		t.Fatalf("got %d pages, first starting %.20q", len(got), got[0])
	}

	if n := len([]rune(got[0])); n > maxMessageLength {
		t.Errorf("page is %d chars, limit %d", n, maxMessageLength)
	}
}

func TestFormatBugListSplitsPages(t *testing.T) {
	var items []bugtracker.Item
	for i := 1; i <= 100; i++ {
		items = append(items, bugtracker.Item{Number: i, Name: strings.Repeat("x", 60)})
	}

	pages := formatBugList(items, false)
	if len(pages) < 2 {
		t.Fatalf("expected multiple pages, got %d", len(pages))
	}

	total := 0
	for i, page := range pages {
		if len(page) > maxMessageLength {
			t.Errorf("page %d is %d chars, limit %d", i, len(page), maxMessageLength)
		}

		total += strings.Count(page, "\n\n")
	}

	if total != len(items) {
		t.Errorf("pages hold %d items, want %d", total, len(items))
	}

	// Numbering continues across pages
	last := pages[len(pages)-1]
	if want := fmt.Sprintf("**%d) ", len(items)); !strings.Contains(last, want) {
		t.Errorf("last page should contain %q", want)
	}
}
