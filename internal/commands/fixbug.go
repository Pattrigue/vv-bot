package commands

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var fixBugCommand = discord.SlashCommandCreate{
	Name:        "fixbug",
	Description: "Mark a bug as fixed (or reopen it)",
	Options: []discord.ApplicationCommandOption{
		bugOption,
		discord.ApplicationCommandOptionBool{
			Name:        "fixed",
			Description: "False to reopen the bug instead (default: true)",
		},
	},
}

func (h *handlers) fixBug(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := e.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	ref := data.String(bugOption.Name)

	fixed, set := data.OptBool("fixed")
	if !set {
		fixed = true
	}

	item, err := h.bugs.SetComplete(ref, fixed)
	if err != nil {
		return fmt.Errorf("updating bug %q: %w", ref, err)
	}

	verb := "reopened"
	if fixed {
		verb = "fixed"
	}

	return editResponse(e, fmt.Sprintf("%s %s bug %s", authorName(e.User(), e.Member()), verb, describe(item)))
}
