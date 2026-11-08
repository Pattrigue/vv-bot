package commands

import (
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
)

const (
	deleteConfirmPrefix = "/deletebug/confirm/"
	deleteCancelID      = "/deletebug/cancel"
)

var deleteBugCommand = discord.SlashCommandCreate{
	Name:        "deletebug",
	Description: "Permanently delete a bug from the bug list",
	Options: []discord.ApplicationCommandOption{
		bugOption,
	},
}

// deleteBug asks for confirmation, visible only to the caller, before anything
// is deleted.
func (h *handlers) deleteBug(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	if err := e.DeferCreateMessage(true); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	ref := data.String(bugOption.Name)

	item, err := h.bugs.Get(ref)
	if err != nil {
		return fmt.Errorf("getting bug %q: %w", ref, err)
	}

	_, err = e.UpdateInteractionResponse(discord.NewMessageUpdate().
		WithContentf("Permanently delete bug %s?", describe(item)).
		AddActionRow(
			discord.NewDangerButton("Delete", deleteConfirmPrefix+item.ID),
			discord.NewSecondaryButton("Cancel", deleteCancelID),
		))
	if err != nil {
		return fmt.Errorf("sending confirmation: %w", err)
	}

	return nil
}

func (h *handlers) confirmDelete(_ discord.ButtonInteractionData, e *handler.ComponentEvent) error {
	if err := e.DeferUpdateMessage(); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	id := e.Vars["id"]

	item, err := h.bugs.Delete(id)
	if err != nil {
		return fmt.Errorf("deleting bug %q: %w", id, err)
	}

	if _, err := e.UpdateInteractionResponse(discord.NewMessageUpdate().WithContent("Deleted.").ClearComponents()); err != nil {
		slog.Error("failed to update confirmation", "err", err)
	}

	// Announce it publicly so there is a record of who deleted what
	followUp(e, fmt.Sprintf("%s deleted bug %s", authorName(e.User(), e.Member()), describe(item)))

	return nil
}

func cancelDelete(_ discord.ButtonInteractionData, e *handler.ComponentEvent) error {
	return e.UpdateMessage(discord.NewMessageUpdate().WithContent("Cancelled. Nothing was deleted.").ClearComponents())
}
