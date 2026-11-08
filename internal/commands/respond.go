package commands

import (
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

const maxMessageLength = 2000 // Discord's limit on a message's content

// responder is an acknowledged interaction that can still be responded to.
type responder interface {
	UpdateInteractionResponse(discord.MessageUpdate, ...rest.RequestOpt) (*discord.Message, error)
	CreateFollowupMessage(discord.MessageCreate, ...rest.RequestOpt) (*discord.Message, error)
}

// editResponse replaces the deferred response with content.
func editResponse(e responder, content string) error {
	_, err := e.UpdateInteractionResponse(discord.NewMessageUpdate().WithContent(truncate(content, maxMessageLength)))

	return err
}

// followUp sends a further message. It only logs failures: the main response
// has already been delivered and must not be replaced by an error.
func followUp(e responder, content string) {
	if _, err := e.CreateFollowupMessage(discord.NewMessageCreate().WithContent(content)); err != nil {
		slog.Error("failed to send followup", "err", err)
	}
}

// handleError logs a handler's error and shows it to the user. Handlers defer
// their response before anything that can fail, so there is one to edit.
func handleError(e *handler.InteractionEvent, err error) {
	slog.Error("failed to handle interaction", "err", err)

	update := discord.NewMessageUpdate().WithContent(bugtracker.UserMessage(err)).ClearComponents()
	if _, err := e.UpdateInteractionResponse(update); err != nil {
		slog.Error("failed to report error to user", "err", err)
	}
}
