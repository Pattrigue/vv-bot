package commands

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

const (
	maxChoices     = 25  // Discord's limit on autocomplete suggestions
	maxChoiceChars = 100 // Discord's limit on a suggestion's label
)

// bugOption is the "bug" option shared by the commands that act on one bug.
// Its value is a Trello item ID chosen from the autocomplete suggestions.
var bugOption = discord.ApplicationCommandOptionString{
	Name:         "bug",
	Description:  "Search by text or list number",
	Required:     true,
	Autocomplete: true,
}

// autocompleteBug suggests bugs matching what the user has typed so far.
func (h *handlers) autocompleteBug(e *handler.AutocompleteEvent) error {
	items, err := h.bugs.List()
	if err != nil {
		slog.Error("failed to list bugs for autocomplete", "err", err)
		return e.AutocompleteResult(nil)
	}

	return e.AutocompleteResult(bugChoices(items, e.Data.String(bugOption.Name)))
}

// bugChoices returns up to maxChoices items matching query, open bugs first
// and newest first. The query matches an item's text or its list number.
func bugChoices(items []bugtracker.Item, query string) []discord.AutocompleteChoice {
	query = strings.ToLower(strings.TrimSpace(query))

	var open, done []discord.AutocompleteChoice

	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		label := choiceLabel(item)

		if !strings.Contains(strings.ToLower(label), query) && !strings.HasPrefix(strconv.Itoa(item.Number), query) {
			continue
		}

		choice := discord.AutocompleteChoiceString{Name: label, Value: item.ID}

		if item.Complete {
			done = append(done, choice)
		} else {
			open = append(open, choice)
		}
	}

	all := append(open, done...)

	return all[:min(len(all), maxChoices)]
}

// choiceLabel renders "12) text", marking fixed bugs. It leaves out the long
// message link so the text fits in Discord's label limit.
func choiceLabel(item bugtracker.Item) string {
	label := fmt.Sprintf("%d) %s", item.Number, item.Summary())
	if item.Complete {
		label = "✅ " + label
	}

	return truncate(label, maxChoiceChars)
}
