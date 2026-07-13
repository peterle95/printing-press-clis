package workflow

import (
	"context"
	"errors"
	"testing"
	"time"
	"trello-calendar-pp-cli/internal/scheduling"
)

type fakeTrello struct {
	cards        []scheduling.Card
	comments     int
	commentError error
}

func (f *fakeTrello) ValidateList(string, string) (string, error)     { return "Doing", nil }
func (f *fakeTrello) ListOpenCards(string) ([]scheduling.Card, error) { return f.cards, nil }
func (f *fakeTrello) AddComment(string, string) error {
	f.comments++
	return f.commentError
}

type fakeCalendar struct {
	events      []scheduling.Event
	existing    map[string]bool
	creates     int
	failCard    string
	accessError error
}

func (f *fakeCalendar) ListEvents(context.Context, time.Time, time.Time) ([]scheduling.Event, error) {
	return append([]scheduling.Event(nil), f.events...), nil
}
func (f *fakeCalendar) FindCard(_ context.Context, _ string, cardID string) (bool, error) {
	return f.existing[cardID], nil
}
func (f *fakeCalendar) CreateEvent(_ context.Context, _ string, assignment scheduling.Assignment, _, _ string) (string, bool, error) {
	f.creates++
	if assignment.Card.ID == f.failCard {
		return "", false, errors.New("injected failure")
	}
	f.existing[assignment.Card.ID] = true
	return "event-" + assignment.Card.ID, true, nil
}
func (f *fakeCalendar) CheckAccess(context.Context) error { return f.accessError }

func testService(t *testing.T, trello *fakeTrello, calendar *fakeCalendar) *Service {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	return &Service{
		Trello: trello, Calendar: calendar,
		Now:     func() time.Time { return time.Date(2026, 7, 12, 12, 0, 0, 0, loc) },
		BoardID: "board", ListID: "list",
		Options: scheduling.Options{Location: loc, DurationMinutes: 60, PreferredTime: "10:00", DayStart: "09:00", DayEnd: "18:00", MaxEventsPerDay: 3},
	}
}

func TestDryRunProducesPlanWithoutWrites(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "card", Name: "Card"}}}
	calendar := &fakeCalendar{existing: map[string]bool{}}
	service := testService(t, trello, calendar)
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result := service.Execute(context.Background(), planned.Plan, true, true)
	if result.Planned != 1 || result.Results[0].Status != "dry-run" {
		t.Fatalf("unexpected dry-run result: %#v", result)
	}
	if calendar.creates != 0 || trello.comments != 0 {
		t.Fatalf("dry-run wrote calendar=%d comments=%d", calendar.creates, trello.comments)
	}
}

func TestPartialCreationFailureContinues(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "a", Name: "A"}, {ID: "b", Name: "B", Position: 1}}}
	calendar := &fakeCalendar{existing: map[string]bool{}, failCard: "a"}
	service := testService(t, trello, calendar)
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result := service.Execute(context.Background(), planned.Plan, false, false)
	if result.Failed != 1 || result.Created != 1 || calendar.creates != 2 {
		t.Fatalf("unexpected partial result: %#v", result)
	}
}

func TestSecondRunDoesNotDuplicate(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "a", Name: "A"}}}
	calendar := &fakeCalendar{existing: map[string]bool{}}
	service := testService(t, trello, calendar)
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := service.Execute(context.Background(), planned.Plan, false, false)
	second := service.Execute(context.Background(), planned.Plan, false, false)
	if first.Created != 1 || second.Existing != 1 || calendar.creates != 1 {
		t.Fatalf("idempotency failed: first=%#v second=%#v creates=%d", first, second, calendar.creates)
	}
}

func TestCommentFailureIsPartial(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "a", Name: "A"}}, commentError: errors.New("comment failed")}
	calendar := &fakeCalendar{existing: map[string]bool{}}
	service := testService(t, trello, calendar)
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result := service.Execute(context.Background(), planned.Plan, false, true)
	if result.Created != 1 || result.Failed != 1 || result.Results[0].Status != "event-created-comment-failed" {
		t.Fatalf("unexpected comment result: %#v", result)
	}
}

func TestEventDescription(t *testing.T) {
	due := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	text := EventDescription(scheduling.Card{ID: "abc", URL: "https://trello.com/c/abc", Due: &due, Labels: []scheduling.Label{{Name: "backend"}, {Name: "priority"}}})
	want := "Scheduled from Trello.\n\nCard: https://trello.com/c/abc\nTrello card ID: abc\nLabels: backend, priority\nDue date: 2026-07-15"
	if text != want {
		t.Fatalf("description:\n%s\nwant:\n%s", text, want)
	}
}
