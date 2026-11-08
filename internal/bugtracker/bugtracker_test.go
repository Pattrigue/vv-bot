package bugtracker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
)

const checklistItems = `[
	{"id":"i1","name":"login broken (Added by Pat) [https://discord.com/channels/1/2/3]","state":"incomplete"},
	{"id":"i2","name":"old crash","state":"complete"}
]`

// fakeTrello serves a canned in-progress list and records write requests.
type fakeTrello struct {
	tracker *Tracker

	mu       sync.Mutex
	requests []string          // "METHOD /path"
	forms    map[string]string // query/form values of the last write request
}

func (f *fakeTrello) wrote(method, path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return slices.Contains(f.requests, method+" "+path)
}

// newFakeTrello starts a server whose list contains the given cards JSON.
func newFakeTrello(t *testing.T, cardsJSON string) *fakeTrello {
	t.Helper()

	f := &fakeTrello{forms: map[string]string{}}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/lists/LIST":
			_, _ = w.Write([]byte(`{"id":"LIST","name":"In Progress"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/lists/LIST/cards":
			_, _ = w.Write([]byte(cardsJSON))
		default:
			_ = r.ParseForm()

			f.mu.Lock()
			f.requests = append(f.requests, r.Method+" "+r.URL.Path)
			for k := range r.Form {
				f.forms[k] = r.Form.Get(k)
			}
			f.mu.Unlock()

			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(server.Close)

	f.tracker = New(Config{APIKey: "key", Token: "token", ListID: "LIST", BaseURL: server.URL})

	return f
}

func card(name string, checklists ...string) map[string]any {
	var lists []json.RawMessage
	for _, c := range checklists {
		lists = append(lists, json.RawMessage(c))
	}

	return map[string]any{"id": "CARD", "name": name, "checklists": lists}
}

func cardsJSON(t *testing.T, cards ...map[string]any) string {
	t.Helper()

	b, err := json.Marshal(cards)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}

func checklistJSON(id string) string {
	return `{"id":"` + id + `","idCard":"CARD","checkItems":` + checklistItems + `}`
}

func standardBoard(t *testing.T) *fakeTrello {
	return newFakeTrello(t, cardsJSON(t, card("Bugs/Issues - Sprint 4", checklistJSON("CL"))))
}

func wantUserError(t *testing.T, err error, contains string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected an error containing %q", contains)
	}

	if !strings.Contains(UserMessage(err), contains) {
		t.Errorf("user message %q should contain %q", UserMessage(err), contains)
	}
}

func TestList(t *testing.T) {
	f := standardBoard(t)

	items, err := f.tracker.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	if items[0].ID != "i1" || items[0].Number != 1 || items[0].Complete {
		t.Errorf("unexpected first item: %+v", items[0])
	}

	if items[1].ID != "i2" || items[1].Number != 2 || !items[1].Complete {
		t.Errorf("unexpected second item: %+v", items[1])
	}
}

func TestGetByIDAndNumber(t *testing.T) {
	f := standardBoard(t)

	for _, ref := range []string{"i2", "2", " 2 "} {
		item, err := f.tracker.Get(ref)
		if err != nil || item.ID != "i2" {
			t.Errorf("Get(%q) = %+v, %v; want item i2", ref, item, err)
		}
	}

	for _, ref := range []string{"nope", "0", "3", ""} {
		_, err := f.tracker.Get(ref)
		wantUserError(t, err, "no longer exists")
	}
}

func TestAdd(t *testing.T) {
	f := standardBoard(t)

	if err := f.tracker.Add("new bug"); err != nil {
		t.Fatal(err)
	}

	if !f.wrote("POST", "/checklists/CL/checkItems") {
		t.Errorf("expected a POST to the checklist's checkItems, got %v", f.requests)
	}

	if f.forms["name"] != "new bug" {
		t.Errorf("name = %q, want %q", f.forms["name"], "new bug")
	}
}

func TestPatchKeepsAuthorAndLink(t *testing.T) {
	f := standardBoard(t)

	before, after, err := f.tracker.Patch("1", "login button missing")
	if err != nil {
		t.Fatal(err)
	}

	wantName := "login button missing (Added by Pat) [https://discord.com/channels/1/2/3]"

	if after.Name != wantName {
		t.Errorf("after.Name = %q, want %q", after.Name, wantName)
	}

	if !strings.HasPrefix(before.Name, "login broken") {
		t.Errorf("before.Name = %q", before.Name)
	}

	if !f.wrote("PUT", "/cards/CARD/checkItem/i1") || f.forms["name"] != wantName {
		t.Errorf("expected a PUT to the card's check item with the new name, got %v %v", f.requests, f.forms)
	}
}

func TestPatchedName(t *testing.T) {
	tests := []struct{ old, report, want string }{
		{"a (Added by Pat) [link]", "b", "b (Added by Pat) [link]"},
		{"plain legacy item", "b", "b"},
		{"x (Added by A) y (Added by B)", "z", "z (Added by B)"}, // keeps the last suffix
	}

	for _, tt := range tests {
		if got := patchedName(tt.old, tt.report); got != tt.want {
			t.Errorf("patchedName(%q, %q) = %q, want %q", tt.old, tt.report, got, tt.want)
		}
	}
}

func TestSetComplete(t *testing.T) {
	f := standardBoard(t)

	item, err := f.tracker.SetComplete("i1", true)
	if err != nil {
		t.Fatal(err)
	}

	if !item.Complete || f.forms["state"] != "complete" {
		t.Errorf("fix: item=%+v state=%q", item, f.forms["state"])
	}

	item, err = f.tracker.SetComplete("i2", false)
	if err != nil {
		t.Fatal(err)
	}

	if item.Complete || f.forms["state"] != "incomplete" {
		t.Errorf("reopen: item=%+v state=%q", item, f.forms["state"])
	}

	if !f.wrote("PUT", "/cards/CARD/checkItem/i2") {
		t.Errorf("expected a PUT for i2, got %v", f.requests)
	}
}

func TestDelete(t *testing.T) {
	f := standardBoard(t)

	item, err := f.tracker.Delete("i2")
	if err != nil {
		t.Fatal(err)
	}

	if item.ID != "i2" || !f.wrote("DELETE", "/checklists/CL/checkItems/i2") {
		t.Errorf("item=%+v requests=%v", item, f.requests)
	}
}

func TestWritesToMissingItemChangeNothing(t *testing.T) {
	f := standardBoard(t)

	_, _, err := f.tracker.Patch("gone", "x")
	wantUserError(t, err, "no longer exists")

	_, err = f.tracker.SetComplete("gone", true)
	wantUserError(t, err, "no longer exists")

	_, err = f.tracker.Delete("gone")
	wantUserError(t, err, "no longer exists")

	if len(f.requests) != 0 {
		t.Errorf("no writes expected, got %v", f.requests)
	}
}

func TestBoardMisconfiguration(t *testing.T) {
	tests := []struct {
		name  string
		cards func(t *testing.T) string
		want  string
	}{
		{
			"no bug card",
			func(t *testing.T) string { return cardsJSON(t, card("Something else", checklistJSON("CL"))) },
			"No active bug list",
		},
		{
			"two bug cards",
			func(t *testing.T) string {
				return cardsJSON(t, card("Bugs/Issues A", checklistJSON("CL")), card("bugs/issues B", checklistJSON("CL2")))
			},
			"Multiple bug cards",
		},
		{
			"two checklists",
			func(t *testing.T) string {
				return cardsJSON(t, card("Bugs/Issues", checklistJSON("CL"), checklistJSON("CL2")))
			},
			"Multiple bug checklists",
		},
		{
			"no checklists",
			func(t *testing.T) string { return cardsJSON(t, card("Bugs/Issues")) },
			"no checklist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeTrello(t, tt.cards(t))

			_, err := f.tracker.List()
			wantUserError(t, err, tt.want)
		})
	}
}

func TestBugCardNameIsCaseInsensitive(t *testing.T) {
	f := newFakeTrello(t, cardsJSON(t, card("BUGS/ISSUES", checklistJSON("CL"))))

	if _, err := f.tracker.List(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUserMessage(t *testing.T) {
	if got := UserMessage(userError("shown")); got != "shown" {
		t.Errorf("got %q", got)
	}

	if got := UserMessage(http.ErrServerClosed); got != "An error occurred!" {
		t.Errorf("got %q", got)
	}
}

func TestItemName(t *testing.T) {
	const link = "https://discord.com/channels/1/2/3"

	if got, want := ItemName("crash", "Pat", link, false), "crash (Added by Pat) ["+link+"]"; got != want {
		t.Errorf("without screenshots: got %q, want %q", got, want)
	}

	if got, want := ItemName("crash", "Pat", link, true), "crash (Added by Pat) [screenshots: "+link+"]"; got != want {
		t.Errorf("with screenshots: got %q, want %q", got, want)
	}
}

func TestItemSummaryAndMaskedName(t *testing.T) {
	tests := []struct{ name, summary, masked string }{
		{"plain", "plain", "plain"},
		{
			"a (Added by P) [https://discord.com/channels/1/2/3]",
			"a (Added by P)",
			"a (Added by P) [[link](https://discord.com/channels/1/2/3)]",
		},
		{
			"a [screenshots: https://discord.com/channels/@me/2/3]",
			"a",
			"a [screenshots: [link](https://discord.com/channels/@me/2/3)]",
		},
	}

	for _, tt := range tests {
		item := Item{Name: tt.name}

		if got := item.Summary(); got != tt.summary {
			t.Errorf("Summary(%q) = %q, want %q", tt.name, got, tt.summary)
		}

		if got := item.MaskedName(); got != tt.masked {
			t.Errorf("MaskedName(%q) = %q, want %q", tt.name, got, tt.masked)
		}
	}
}
