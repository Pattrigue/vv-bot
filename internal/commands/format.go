package commands

import (
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/discord"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

// authorName is the display name of whoever ran the command.
func authorName(user discord.User, member *discord.ResolvedMember) string {
	if member != nil {
		return member.EffectiveName()
	}

	return user.Username
}

// cleanReport collapses all whitespace runs in a report to single spaces.
func cleanReport(report string) string {
	return strings.Join(strings.Fields(report), " ")
}

// describe renders an item as "#12 text" for public announcements.
func describe(item bugtracker.Item) string {
	return fmt.Sprintf("#%d %s", item.Number, item.Summary())
}

// truncate shortens s to at most limit characters, marking any cut with "…".
func truncate(s string, limit int) string {
	if runes := []rune(s); len(runes) > limit {
		return string(runes[:limit-1]) + "…"
	}

	return s
}
