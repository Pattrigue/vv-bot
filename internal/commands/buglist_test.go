package commands

import (
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

		want := "- **first**\n- ~~second~~\n- **third**\n"
		if len(got) != 1 || got[0] != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("incomplete only", func(t *testing.T) {
		got := formatBugList(items, true)

		want := "- **first**\n- **third**\n"
		if len(got) != 1 || got[0] != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("message links are masked", func(t *testing.T) {
		link := "https://discord.com/channels/1/2/3"
		got := formatBugList([]bugtracker.Item{{Number: 1, Name: "bug [" + link + "]"}}, false)

		if want := "[link](<" + link + ">)"; !strings.Contains(got[0], want) {
			t.Errorf("got %q, want it to contain %q", got[0], want)
		}
	})

	t.Run("multi-line items stay on one line", func(t *testing.T) {
		got := formatBugList([]bugtracker.Item{{Number: 1, Name: "first\nsecond", Complete: true}}, false)

		if want := "- ~~first second~~\n"; got[0] != want {
			t.Errorf("got %q, want %q", got[0], want)
		}
	})

	t.Run("other links do not embed", func(t *testing.T) {
		tests := map[string]string{
			"see https://example.com/a now":   "see <https://example.com/a> now",
			"see <https://example.com/a> now": "see <https://example.com/a> now",
			"see [a](https://example.com/a)":  "see [a](<https://example.com/a>)",
		}

		for name, want := range tests {
			got := formatBugList([]bugtracker.Item{{Number: 1, Name: name}}, false)
			if !strings.Contains(got[0], want) {
				t.Errorf("got %q, want it to contain %q", got[0], want)
			}
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
	if len(got) != 1 || !strings.HasPrefix(got[0], "- **xxx") {
		t.Fatalf("got %d pages, first starting %.20q", len(got), got[0])
	}

	if !strings.HasSuffix(got[0], "**\n") {
		t.Errorf("closing markers were cut off: ends %q", got[0][len(got[0])-10:])
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

		total += strings.Count(page, "\n")
	}

	if total != len(items) {
		t.Errorf("pages hold %d items, want %d", total, len(items))
	}
}
