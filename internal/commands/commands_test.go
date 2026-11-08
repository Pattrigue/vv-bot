package commands

import (
	"testing"

	"github.com/disgoorg/disgo/discord"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

func TestDescribe(t *testing.T) {
	item := bugtracker.Item{Number: 7, Name: "crash (Added by Pat) [https://discord.com/channels/1/2/3]"}

	if got, want := describe(item), "#7 crash (Added by Pat)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAuthorName(t *testing.T) {
	user := discord.User{Username: "pat"}

	if got := authorName(user, nil); got != "pat" {
		t.Errorf("without member: got %q, want %q", got, "pat")
	}

	member := &discord.ResolvedMember{Member: discord.Member{Nick: new("Patrick")}}
	if got := authorName(user, member); got != "Patrick" {
		t.Errorf("with nickname: got %q, want %q", got, "Patrick")
	}
}

func TestDefinitions(t *testing.T) {
	router := NewRouter(nil)
	seen := map[string]bool{}

	for i, def := range Definitions {
		cmd, ok := def.(discord.SlashCommandCreate)
		if !ok {
			t.Fatalf("command %d is not a slash command", i)
		}

		if seen[cmd.Name] {
			t.Errorf("duplicate command %q", cmd.Name)
		}
		seen[cmd.Name] = true

		if cmd.Description == "" {
			t.Errorf("%s has no description", cmd.Name)
		}

		if !router.Match("/"+cmd.Name, discord.InteractionTypeApplicationCommand, int(discord.ApplicationCommandTypeSlash)) {
			t.Errorf("%s has no route", cmd.Name)
		}

		// Discord rejects commands where a required option follows an optional one
		optionalSeen := false
		for _, opt := range cmd.Options {
			if !isRequired(opt) {
				optionalSeen = true
			} else if optionalSeen {
				t.Errorf("%s: required option %q follows an optional one", cmd.Name, opt.OptionName())
			}

			if s, ok := opt.(discord.ApplicationCommandOptionString); ok && s.Autocomplete &&
				!router.Match("/"+cmd.Name, discord.InteractionTypeAutocomplete, 0) {
				t.Errorf("%s: option %q has no autocomplete route", cmd.Name, opt.OptionName())
			}
		}
	}
}

func isRequired(opt discord.ApplicationCommandOption) bool {
	switch o := opt.(type) {
	case discord.ApplicationCommandOptionString:
		return o.Required
	case discord.ApplicationCommandOptionBool:
		return o.Required
	case discord.ApplicationCommandOptionAttachment:
		return o.Required
	}

	return false
}

// patchbug, fixbug and deletebug all rely on the shared autocompleting bug option.
func TestBugCommandsUseAutocompletingBugOption(t *testing.T) {
	if !bugOption.Autocomplete || !bugOption.Required {
		t.Fatal("bugOption must be a required, autocompleting option")
	}

	for _, cmd := range []discord.SlashCommandCreate{patchBugCommand, fixBugCommand, deleteBugCommand} {
		first, ok := cmd.Options[0].(discord.ApplicationCommandOptionString)
		if !ok || first.Name != bugOption.Name || !first.Autocomplete {
			t.Errorf("%s should take bugOption first", cmd.Name)
		}
	}
}
