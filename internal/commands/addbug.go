package commands

import (
	"fmt"
	"log/slog"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

const (
	maxScreenshots = 3

	// maxReportLength leaves room within maxMessageLength for the text the bot
	// adds around a report.
	maxReportLength = 1500
)

var addBugCommand = discord.SlashCommandCreate{
	Name:        "addbug",
	Description: "Add a bug report to the bug list",
	Options:     addBugOptions(),
}

func addBugOptions() []discord.ApplicationCommandOption {
	opts := []discord.ApplicationCommandOption{reportOption("Describe the bug")}

	for i := 1; i <= maxScreenshots; i++ {
		opts = append(opts, discord.ApplicationCommandOptionAttachment{
			Name:        screenshotOptionName(i),
			Description: "Screenshot of the bug",
		})
	}

	return opts
}

func reportOption(description string) discord.ApplicationCommandOptionString {
	return discord.ApplicationCommandOptionString{
		Name:        "report",
		Description: description,
		Required:    true,
		MaxLength:   new(maxReportLength),
	}
}

func screenshotOptionName(i int) string {
	return fmt.Sprintf("screenshot%d", i)
}

func (h *handlers) addBug(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	report := cleanReport(data.String("report"))
	author := authorName(e.User(), e.Member())

	var embeds []discord.Embed

	for i := 1; i <= maxScreenshots; i++ {
		if att, ok := data.OptAttachment(screenshotOptionName(i)); ok {
			embeds = append(embeds, discord.NewEmbed().WithImage(att.URL))
		}
	}

	if err := e.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	// Post the report (with screenshots) first so we have a message to link to.
	msg, err := e.UpdateInteractionResponse(discord.NewMessageUpdate().
		WithContentf("**Bug report from %s:**\n%s", author, report).
		WithEmbeds(embeds...))
	if err != nil {
		return fmt.Errorf("posting bug report: %w", err)
	}

	// The post stays up either way, so a failure is reported in a followup.
	if err := h.bugs.Add(bugtracker.ItemName(report, author, msg.JumpURL(), len(embeds) > 0)); err != nil {
		slog.Error("failed to add bug", "err", err, "report", report)
		followUp(e, bugtracker.UserMessage(err))
	}

	return nil
}
