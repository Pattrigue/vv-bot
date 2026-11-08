package commands

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

var bugListCommand = discord.SlashCommandCreate{
	Name:        "buglist",
	Description: "Show the items on the bug list",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionBool{
			Name:        "incomplete",
			Description: "Only show incomplete bugs",
		},
	},
}

func (h *handlers) bugList(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := e.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	items, err := h.bugs.List()
	if err != nil {
		return fmt.Errorf("listing bugs: %w", err)
	}

	pages := formatBugList(items, data.Bool("incomplete"))

	if err := editResponse(e, pages[0]); err != nil {
		return fmt.Errorf("sending bug list: %w", err)
	}

	for _, page := range pages[1:] {
		followUp(e, page)
	}

	return nil
}

// formatBugList renders the items as Discord messages, each within the
// message length limit.
func formatBugList(items []bugtracker.Item, incompleteOnly bool) []string {
	var pages []string

	for _, item := range items {
		if incompleteOnly && item.Complete {
			continue
		}

		format := "- **%s**\n"
		if item.Complete {
			format = "- ~~%s~~\n"
		}

		name := truncate(cleanReport(item.MaskedName()), maxMessageLength-len(format))
		line := fmt.Sprintf(format, name)

		if last := len(pages) - 1; last >= 0 && len(pages[last])+len(line) <= maxMessageLength {
			pages[last] += line
		} else {
			pages = append(pages, line)
		}
	}

	if len(pages) == 0 {
		return []string{"No bugs found!"}
	}

	return pages
}
