package commands

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

var patchBugCommand = discord.SlashCommandCreate{
	Name:        "patchbug",
	Description: "Replace the text of a bug report",
	Options: []discord.ApplicationCommandOption{
		bugOption,
		reportOption("The new bug description"),
	},
}

func (h *handlers) patchBug(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := e.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	ref := data.String(bugOption.Name)

	before, after, err := h.bugs.Patch(ref, cleanReport(data.String("report")))
	if err != nil {
		return fmt.Errorf("patching bug %q: %w", ref, err)
	}

	return editResponse(e, fmt.Sprintf("%s patched bug %s\nWas: ~~%s~~\nNow: %s",
		authorName(e.User(), e.Member()), describe(after), describe(before), describe(after)))
}
