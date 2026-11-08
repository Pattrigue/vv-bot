package commands

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/handler"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

const (
	maxAttachments = 3

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

	for i := 1; i <= maxAttachments; i++ {
		opts = append(opts, discord.ApplicationCommandOptionAttachment{
			Name:        attachmentOptionName(i),
			Description: "Screenshot or video of the bug",
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

func attachmentOptionName(i int) string {
	return fmt.Sprintf("attachment%d", i)
}

func (h *handlers) addBug(data discord.SlashCommandInteractionData, e *handler.CommandEvent) error {
	report := cleanReport(data.String("report"))
	author := authorName(e.User(), e.Member())

	var attachments []discord.Attachment

	for i := 1; i <= maxAttachments; i++ {
		if att, ok := data.OptAttachment(attachmentOptionName(i)); ok {
			attachments = append(attachments, att)
		}
	}

	if err := e.DeferCreateMessage(false); err != nil {
		return fmt.Errorf("deferring response: %w", err)
	}

	media := collectMedia(attachments, e.AttachmentSizeLimit())
	content := fmt.Sprintf("**Bug report from %s:**\n%s", author, report)
	update, overflow := media.update(content)

	// Post the report (with attachments) first so we have a message to link to.
	msg, err := e.UpdateInteractionResponse(update)
	if err != nil && len(media.uploads) > 0 {
		// An upload can time out or be refused. Links still get the report posted.
		slog.Error("failed to upload videos, linking them instead", "err", err)

		update, overflow = media.withoutUploads().update(content)
		msg, err = e.UpdateInteractionResponse(update)
	}

	if err != nil {
		return fmt.Errorf("posting bug report: %w", err)
	}

	if len(overflow) > 0 {
		followUp(e, strings.Join(overflow, "\n"))
	}

	// The post stays up either way, so a failure is reported in a followup.
	if err := h.bugs.Add(bugtracker.ItemName(report, author, msg.JumpURL(), len(attachments) > 0)); err != nil {
		slog.Error("failed to add bug", "err", err, "report", report)
		followUp(e, bugtracker.UserMessage(err))
	}

	return nil
}
