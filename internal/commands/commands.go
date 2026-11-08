// Package commands defines the bot's slash commands and routes interactions
// to their handlers.
package commands

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

// Definitions lists every slash command to register with Discord.
var Definitions = []discord.ApplicationCommandCreate{
	pingCommand,
	addBugCommand,
	bugListCommand,
	patchBugCommand,
	fixBugCommand,
	deleteBugCommand,
}

// handlers holds the dependencies of the command handlers.
type handlers struct {
	bugs *bugtracker.Tracker
}

// NewRouter returns the event listener that routes every interaction.
func NewRouter(bugs *bugtracker.Tracker) *handler.Mux {
	h := &handlers{bugs: bugs}

	r := handler.New()
	r.Error(handleError)

	r.SlashCommand("/ping", handlePing)
	r.SlashCommand("/addbug", h.addBug)
	r.SlashCommand("/buglist", h.bugList)
	r.SlashCommand("/patchbug", h.patchBug)
	r.SlashCommand("/fixbug", h.fixBug)
	r.SlashCommand("/deletebug", h.deleteBug)

	r.Autocomplete("/patchbug", h.autocompleteBug)
	r.Autocomplete("/fixbug", h.autocompleteBug)
	r.Autocomplete("/deletebug", h.autocompleteBug)

	r.ButtonComponent(deleteConfirmPrefix+"{id}", h.confirmDelete)
	r.ButtonComponent(deleteCancelID, cancelDelete)

	return r
}
