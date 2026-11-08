package commands

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/handler"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/Pattrigue/VVBot/internal/bugtracker"
)

// fakeDiscord records what the handlers send to Discord.
type fakeDiscord struct {
	rest.Rest

	responses []discord.InteractionResponseType
	data      []discord.InteractionResponseData
	edits     []discord.MessageUpdate
	followups []discord.MessageCreate
}

func (f *fakeDiscord) respond(t discord.InteractionResponseType, data discord.InteractionResponseData, _ ...rest.RequestOpt) error {
	f.responses = append(f.responses, t)
	f.data = append(f.data, data)

	return nil
}

func (f *fakeDiscord) UpdateInteractionResponse(_ snowflake.ID, _ string, update discord.MessageUpdate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.edits = append(f.edits, update)

	return &discord.Message{ID: 9, ChannelID: 8}, nil
}

func (f *fakeDiscord) CreateFollowupMessage(_ snowflake.ID, _ string, msg discord.MessageCreate, _ ...rest.RequestOpt) (*discord.Message, error) {
	f.followups = append(f.followups, msg)

	return &discord.Message{}, nil
}

func (f *fakeDiscord) lastEdit(t *testing.T) discord.MessageUpdate {
	t.Helper()

	if len(f.edits) == 0 {
		t.Fatal("no response edits were sent")
	}

	return f.edits[len(f.edits)-1]
}

// fakeTrello serves one bug card and records write requests. While failWrites
// is set, writes fail.
type fakeTrello struct {
	mu         sync.Mutex
	writes     []string
	failWrites bool
}

func (f *fakeTrello) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/lists/LIST":
		fmt.Fprint(w, `{"id":"LIST"}`)
	case r.Method == http.MethodGet && r.URL.Path == "/lists/LIST/cards":
		fmt.Fprint(w, `[{"id":"CARD","name":"Bugs/Issues","checklists":[{"id":"CL","checkItems":[
			{"id":"i1","name":"login broken (Added by Pat) [https://discord.com/channels/1/2/3]","state":"incomplete"},
			{"id":"i2","name":"old crash","state":"complete"}
		]}]}]`)
	default:
		f.mu.Lock()
		defer f.mu.Unlock()

		f.writes = append(f.writes, r.Method+" "+r.URL.Path)

		if f.failWrites {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		fmt.Fprint(w, `{}`)
	}
}

type testBot struct {
	router  *handler.Mux
	discord *fakeDiscord
	trello  *fakeTrello
}

func newTestBot(t *testing.T) *testBot {
	t.Helper()

	trello := &fakeTrello{}

	server := httptest.NewServer(trello)
	t.Cleanup(server.Close)

	bugs := bugtracker.New(bugtracker.Config{APIKey: "key", Token: "token", ListID: "LIST", BaseURL: server.URL})

	return &testBot{router: NewRouter(bugs), discord: &fakeDiscord{}, trello: trello}
}

// dispatch sends an interaction, given as its type and data JSON, to the router.
func (b *testBot) dispatch(t *testing.T, interactionType discord.InteractionType, dataJSON string) {
	t.Helper()

	raw := fmt.Sprintf(`{"id":"1","application_id":"2","token":"tok","version":1,"type":%d,
		"user":{"id":"3","username":"pat"},
		"message":{"id":"4","channel_id":"5"},
		"data":%s}`, interactionType, dataJSON)

	interaction, err := discord.UnmarshalInteraction([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}

	b.router.OnEvent(&events.InteractionCreate{
		GenericEvent: events.NewGenericEvent(&bot.Client{Rest: b.discord}, 0, 0),
		Interaction:  interaction,
		Respond:      b.discord.respond,
	})
}

func (b *testBot) command(t *testing.T, name, optionsJSON string) {
	t.Helper()

	b.dispatch(t, discord.InteractionTypeApplicationCommand,
		fmt.Sprintf(`{"id":"6","type":1,"name":%q,"options":%s}`, name, optionsJSON))
}

func (b *testBot) click(t *testing.T, customID string) {
	t.Helper()

	b.dispatch(t, discord.InteractionTypeComponent, fmt.Sprintf(`{"component_type":2,"custom_id":%q}`, customID))
}

func TestPing(t *testing.T) {
	b := newTestBot(t)
	b.command(t, "ping", `[]`)

	if len(b.discord.data) != 1 || b.discord.data[0].(discord.MessageCreate).Content != "pong" {
		t.Errorf("responses = %+v", b.discord.data)
	}
}

func TestFixBug(t *testing.T) {
	b := newTestBot(t)
	b.command(t, "fixbug", `[{"name":"bug","type":3,"value":"i1"}]`)

	if !slices.Contains(b.trello.writes, "PUT /cards/CARD/checkItem/i1") {
		t.Errorf("writes = %v", b.trello.writes)
	}

	if b.discord.responses[0] != discord.InteractionResponseTypeDeferredCreateMessage {
		t.Errorf("first response = %v, want a defer", b.discord.responses[0])
	}

	if got, want := *b.discord.lastEdit(t).Content, "pat fixed bug #1 login broken (Added by Pat)"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPatchBug(t *testing.T) {
	b := newTestBot(t)
	b.command(t, "patchbug", `[{"name":"bug","type":3,"value":"i1"},{"name":"report","type":3,"value":"  new   text "}]`)

	got := *b.discord.lastEdit(t).Content
	for _, want := range []string{"Was: ~~#1 login broken (Added by Pat)~~", "Now: #1 new text (Added by Pat)"} {
		if !strings.Contains(got, want) {
			t.Errorf("got %q, want it to contain %q", got, want)
		}
	}
}

func TestBugList(t *testing.T) {
	b := newTestBot(t)
	b.command(t, "buglist", `[{"name":"incomplete","type":5,"value":true}]`)

	got := *b.discord.lastEdit(t).Content
	if !strings.Contains(got, "**1) login broken") || strings.Contains(got, "old crash") {
		t.Errorf("got %q", got)
	}
}

func TestErrorsAreShownInTheResponse(t *testing.T) {
	tests := []struct{ name, command, options string }{
		{"fixbug", "fixbug", `[{"name":"bug","type":3,"value":"gone"}]`},
		{"patchbug", "patchbug", `[{"name":"bug","type":3,"value":"gone"},{"name":"report","type":3,"value":"x"}]`},
		{"deletebug", "deletebug", `[{"name":"bug","type":3,"value":"gone"}]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newTestBot(t)
			b.command(t, tt.command, tt.options)

			if got := *b.discord.lastEdit(t).Content; !strings.Contains(got, "no longer exists") {
				t.Errorf("got %q", got)
			}

			if len(b.trello.writes) != 0 {
				t.Errorf("writes = %v", b.trello.writes)
			}
		})
	}
}

func TestInternalErrorsAreNotShown(t *testing.T) {
	b := newTestBot(t)
	b.trello.failWrites = true
	b.command(t, "fixbug", `[{"name":"bug","type":3,"value":"i1"}]`)

	if got := *b.discord.lastEdit(t).Content; got != "An error occurred!" {
		t.Errorf("got %q", got)
	}
}

func TestAddBug(t *testing.T) {
	b := newTestBot(t)
	b.command(t, "addbug", `[{"name":"report","type":3,"value":"it broke"}]`)

	if !slices.Contains(b.trello.writes, "POST /checklists/CL/checkItems") {
		t.Errorf("writes = %v", b.trello.writes)
	}

	if got := *b.discord.lastEdit(t).Content; got != "**Bug report from pat:**\nit broke" {
		t.Errorf("post = %q", got)
	}

	if len(b.discord.followups) != 1 || !strings.Contains(b.discord.followups[0].Content, "successfully") {
		t.Errorf("followups = %+v", b.discord.followups)
	}
}

func TestAddBugFailureKeepsThePost(t *testing.T) {
	b := newTestBot(t)
	b.trello.failWrites = true
	b.command(t, "addbug", `[{"name":"report","type":3,"value":"it broke"}]`)

	if len(b.discord.edits) != 1 || !strings.Contains(*b.discord.edits[0].Content, "it broke") {
		t.Errorf("edits = %+v", b.discord.edits)
	}

	if len(b.discord.followups) != 1 || b.discord.followups[0].Content != "An error occurred!" {
		t.Errorf("followups = %+v", b.discord.followups)
	}
}

func TestDeleteBugAsksFirst(t *testing.T) {
	b := newTestBot(t)
	b.command(t, "deletebug", `[{"name":"bug","type":3,"value":"i1"}]`)

	if len(b.trello.writes) != 0 {
		t.Errorf("nothing should be deleted yet, writes = %v", b.trello.writes)
	}

	deferred := b.discord.data[0].(discord.MessageCreate)
	if deferred.Flags != discord.MessageFlagEphemeral {
		t.Errorf("confirmation should be ephemeral, flags = %v", deferred.Flags)
	}

	edit := b.discord.lastEdit(t)
	if !strings.Contains(*edit.Content, "#1 login broken") || edit.Components == nil || len(*edit.Components) != 1 {
		t.Errorf("edit = %+v", edit)
	}
}

func TestConfirmDelete(t *testing.T) {
	b := newTestBot(t)
	b.click(t, deleteConfirmPrefix+"i1")

	if !slices.Contains(b.trello.writes, "DELETE /checklists/CL/checkItems/i1") {
		t.Errorf("writes = %v", b.trello.writes)
	}

	if b.discord.responses[0] != discord.InteractionResponseTypeDeferredUpdateMessage {
		t.Errorf("first response = %v, want a defer", b.discord.responses[0])
	}

	edit := b.discord.lastEdit(t)
	if *edit.Content != "Deleted." || edit.Components == nil || len(*edit.Components) != 0 {
		t.Errorf("edit = %+v", edit)
	}

	if len(b.discord.followups) != 1 || b.discord.followups[0].Content != "pat deleted bug #1 login broken (Added by Pat)" {
		t.Errorf("followups = %+v", b.discord.followups)
	}
}

func TestConfirmDeleteOfMissingBugRemovesButtons(t *testing.T) {
	b := newTestBot(t)
	b.click(t, deleteConfirmPrefix+"gone")

	edit := b.discord.lastEdit(t)
	if !strings.Contains(*edit.Content, "no longer exists") || edit.Components == nil || len(*edit.Components) != 0 {
		t.Errorf("edit = %+v", edit)
	}
}

func TestCancelDelete(t *testing.T) {
	b := newTestBot(t)
	b.click(t, deleteCancelID)

	if len(b.trello.writes) != 0 {
		t.Errorf("writes = %v", b.trello.writes)
	}

	if b.discord.responses[0] != discord.InteractionResponseTypeUpdateMessage {
		t.Errorf("response = %v", b.discord.responses[0])
	}
}

func TestAutocomplete(t *testing.T) {
	for _, command := range []string{"patchbug", "fixbug", "deletebug"} {
		b := newTestBot(t)
		b.dispatch(t, discord.InteractionTypeAutocomplete,
			fmt.Sprintf(`{"id":"6","name":%q,"options":[{"name":"bug","type":3,"value":"login","focused":true}]}`, command))

		if len(b.discord.responses) != 1 || b.discord.responses[0] != discord.InteractionResponseTypeAutocompleteResult {
			t.Fatalf("%s: responses = %v", command, b.discord.responses)
		}

		result := b.discord.data[0].(discord.AutocompleteResult)
		if len(result.Choices) != 1 || result.Choices[0].(discord.AutocompleteChoiceString).Value != "i1" {
			t.Errorf("%s: choices = %+v", command, result.Choices)
		}
	}
}
