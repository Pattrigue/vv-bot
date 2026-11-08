// Package bugtracker reads and writes the bug list kept as a checklist on the
// active "Bugs/Issues" Trello card.
package bugtracker

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adlio/trello"
)

const (
	bugCardPrefix  = "bugs/issues"
	requestTimeout = 10 * time.Second
)

// Config holds what a Tracker needs to reach the bug list.
type Config struct {
	APIKey  string
	Token   string
	ListID  string // the Trello list holding the active bug card
	BaseURL string // defaults to the public Trello API
}

// Tracker gives access to the bug list.
type Tracker struct {
	client *trello.Client
	listID string
}

// New returns a Tracker for cfg.
func New(cfg Config) *Tracker {
	client := trello.NewClient(cfg.APIKey, cfg.Token)
	client.Client = &http.Client{Timeout: requestTimeout}

	if cfg.BaseURL != "" {
		client.BaseURL = cfg.BaseURL
	}

	return &Tracker{client: client, listID: cfg.ListID}
}

// userError is an error whose message is safe to show to Discord users.
type userError string

func (e userError) Error() string { return string(e) }

// UserMessage returns the text to show a Discord user for err.
func UserMessage(err error) string {
	if ue, ok := errors.AsType[userError](err); ok {
		return string(ue)
	}

	return "An error occurred!"
}

// List returns every item on the bug list, in checklist order.
func (t *Tracker) List() ([]Item, error) {
	checklist, err := t.checklist()
	if err != nil {
		return nil, err
	}

	return itemsOf(checklist), nil
}

// Get returns the item identified by ref, which is a Trello item ID or a
// list number.
func (t *Tracker) Get(ref string) (Item, error) {
	_, item, err := t.lookup(ref)

	return item, err
}

// Add appends a new item to the bug list.
func (t *Tracker) Add(name string) error {
	checklist, err := t.checklist()
	if err != nil {
		return err
	}

	if _, err := checklist.CreateCheckItem(name); err != nil {
		return fmt.Errorf("creating check item: %w", err)
	}

	return nil
}

// Patch replaces the report text of an item while keeping who added it and
// the link to the original message. It returns the item as it was before.
func (t *Tracker) Patch(ref, report string) (before, after Item, err error) {
	checklist, before, err := t.lookup(ref)
	if err != nil {
		return Item{}, Item{}, err
	}

	after = before
	after.Name = patchedName(before.Name, report)

	if err := t.updateItem(checklist, before.ID, trello.Arguments{"name": after.Name}); err != nil {
		return Item{}, Item{}, err
	}

	return before, after, nil
}

// SetComplete marks an item as fixed or reopens it.
func (t *Tracker) SetComplete(ref string, complete bool) (Item, error) {
	checklist, item, err := t.lookup(ref)
	if err != nil {
		return Item{}, err
	}

	state := "incomplete"
	if complete {
		state = "complete"
	}

	if err := t.updateItem(checklist, item.ID, trello.Arguments{"state": state}); err != nil {
		return Item{}, err
	}

	item.Complete = complete

	return item, nil
}

// Delete permanently removes an item and returns what was deleted.
func (t *Tracker) Delete(ref string) (Item, error) {
	checklist, item, err := t.lookup(ref)
	if err != nil {
		return Item{}, err
	}

	path := "checklists/" + checklist.ID + "/checkItems/" + item.ID
	if err := t.client.Delete(path, trello.Defaults(), &struct{}{}); err != nil {
		return Item{}, fmt.Errorf("deleting check item: %w", err)
	}

	return item, nil
}

func (t *Tracker) updateItem(checklist *trello.Checklist, itemID string, args trello.Arguments) error {
	path := "cards/" + checklist.IDCard + "/checkItem/" + itemID
	if err := t.client.Put(path, args, &struct{}{}); err != nil {
		return fmt.Errorf("updating check item: %w", err)
	}

	return nil
}

// lookup returns the active checklist along with the item identified by ref.
func (t *Tracker) lookup(ref string) (*trello.Checklist, Item, error) {
	checklist, err := t.checklist()
	if err != nil {
		return nil, Item{}, err
	}

	item, err := find(itemsOf(checklist), ref)
	if err != nil {
		return nil, Item{}, err
	}

	return checklist, item, nil
}

// checklist returns the single checklist of the single active bug card.
func (t *Tracker) checklist() (*trello.Checklist, error) {
	list, err := t.client.GetList(t.listID, trello.Defaults())
	if err != nil {
		return nil, fmt.Errorf("getting trello list: %w", err)
	}

	cards, err := list.GetCards(trello.Arguments{"checklists": "all"})
	if err != nil {
		return nil, fmt.Errorf("getting trello cards: %w", err)
	}

	var bugCard *trello.Card

	for _, card := range cards {
		if !strings.HasPrefix(strings.ToLower(card.Name), bugCardPrefix) {
			continue
		}

		if bugCard != nil {
			return nil, userError("Multiple bug cards active! That's a mistake.\nPlease contact Pattrigue or aqitf.")
		}

		bugCard = card
	}

	if bugCard == nil {
		return nil, userError("No active bug list found, please contact Pattrigue or aqitf.")
	}

	switch len(bugCard.Checklists) {
	case 0:
		return nil, userError("The active bug card has no checklist! That's a mistake.\nPlease contact Pattrigue.")
	case 1:
	default:
		return nil, userError("Multiple bug checklists are active! That's a mistake.\nPlease contact Pattrigue.")
	}

	checklist := bugCard.Checklists[0]
	checklist.IDCard = bugCard.ID // needed to update items through the card endpoint

	return checklist, nil
}

func itemsOf(checklist *trello.Checklist) []Item {
	items := make([]Item, len(checklist.CheckItems))
	for i, item := range checklist.CheckItems {
		items[i] = Item{
			ID:       item.ID,
			Number:   i + 1,
			Name:     item.Name,
			Complete: item.State == "complete",
		}
	}

	return items
}

// find looks an item up by Trello ID, falling back to its list number.
func find(items []Item, ref string) (Item, error) {
	for _, item := range items {
		if item.ID == ref {
			return item, nil
		}
	}

	if n, err := strconv.Atoi(strings.TrimSpace(ref)); err == nil && n >= 1 && n <= len(items) {
		return items[n-1], nil
	}

	return Item{}, userError("That bug no longer exists. Check /buglist and try again.")
}
