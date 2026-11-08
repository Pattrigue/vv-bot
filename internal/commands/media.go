package commands

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/disgoorg/disgo/discord"
)

const downloadTimeout = 60 * time.Second

var downloadClient = &http.Client{Timeout: downloadTimeout}

// upload is a downloaded video to attach to the report's post.
type upload struct {
	filename string
	url      string
	data     []byte
}

// reportMedia is how a report's attachments appear in its post. Images are
// embedded. Videos can't be, so they are re-uploaded, or linked when that isn't
// possible.
type reportMedia struct {
	embeds  []discord.Embed
	uploads []upload
	links   []string
}

// collectMedia sorts attachments into reportMedia, downloading videos that fit
// within uploadLimit, the number of bytes the post may carry in total.
func collectMedia(attachments []discord.Attachment, uploadLimit int) reportMedia {
	var media reportMedia

	for _, att := range attachments {
		if !isVideo(att) {
			media.embeds = append(media.embeds, discord.NewEmbed().WithImage(att.URL))
			continue
		}

		if att.Size > uploadLimit {
			media.links = append(media.links, att.URL)
			continue
		}

		data, err := download(att.URL)
		if err != nil {
			slog.Error("failed to download video", "err", err, "filename", att.Filename)
			media.links = append(media.links, att.URL)

			continue
		}

		uploadLimit -= len(data)
		media.uploads = append(media.uploads, upload{filename: att.Filename, url: att.URL, data: data})
	}

	return media
}

func isVideo(att discord.Attachment) bool {
	return att.ContentType != nil && strings.HasPrefix(*att.ContentType, "video/")
}

func download(url string) ([]byte, error) {
	resp, err := downloadClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// update builds the post from content, returning any links that didn't fit in
// the message.
func (m reportMedia) update(content string) (discord.MessageUpdate, []string) {
	content, overflow := appendLinks(content, m.links)
	update := discord.NewMessageUpdate().WithContent(content).WithEmbeds(m.embeds...)

	for _, u := range m.uploads {
		update = update.AddFile(u.filename, "", bytes.NewReader(u.data))
	}

	return update, overflow
}

// withoutUploads returns the media with every upload replaced by a link.
func (m reportMedia) withoutUploads() reportMedia {
	for _, u := range m.uploads {
		m.links = append(m.links, u.url)
	}

	m.uploads = nil

	return m
}

// appendLinks adds each link on its own line while the message stays within
// Discord's limit, since a truncated link would be useless.
func appendLinks(content string, links []string) (string, []string) {
	var overflow []string

	for _, link := range links {
		if utf8.RuneCountInString(content)+1+utf8.RuneCountInString(link) > maxMessageLength {
			overflow = append(overflow, link)
			continue
		}

		content += "\n" + link
	}

	return content, overflow
}
