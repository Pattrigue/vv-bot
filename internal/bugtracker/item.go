package bugtracker

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// addedBySeparator starts the author and message link suffix of an item
	// name. Patching a report keeps everything from this point on.
	addedBySeparator = " (Added by "

	messageLinkPrefix = `https://discord\.com/channels/`
)

var (
	messageLinkRe       = regexp.MustCompile(messageLinkPrefix + `(?:@me|\d+)(?:/\d+)+`)
	messageLinkSuffixRe = regexp.MustCompile(`\s*\[[^\]]*` + messageLinkPrefix + `[^\]]*\]`)
)

// Item is a single entry on the bug list.
type Item struct {
	ID       string
	Number   int // 1-based position in the checklist, as shown by /buglist
	Name     string
	Complete bool
}

// Summary is the item's name without its message link.
func (i Item) Summary() string {
	return messageLinkSuffixRe.ReplaceAllString(i.Name, "")
}

// MaskedName is the item's name with its message link as a short Markdown link.
func (i Item) MaskedName() string {
	return messageLinkRe.ReplaceAllString(i.Name, "[link]($0)")
}

// ItemName builds the name of a new item. It links back to the Discord message
// so that any screenshots are reachable from Trello.
func ItemName(report, author, messageLink string, hasScreenshots bool) string {
	name := report + addedBySeparator + author + ")"

	if hasScreenshots {
		return fmt.Sprintf("%s [screenshots: %s]", name, messageLink)
	}

	return fmt.Sprintf("%s [%s]", name, messageLink)
}

// patchedName swaps the report text in an item name for a new one, keeping the
// author and message link suffix if present.
func patchedName(old, report string) string {
	if i := strings.LastIndex(old, addedBySeparator); i >= 0 {
		return report + old[i:]
	}

	return report
}
