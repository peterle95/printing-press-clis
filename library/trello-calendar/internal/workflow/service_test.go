package workflow

import (
	"context"
	"errors"
	"testing"
	"time"
	"trello-calendar-pp-cli/internal/scheduling"
	trelloadapter "trello-calendar-pp-cli/internal/trello"
)

type fakeTrello struct {
	cards        []scheduling.Card
	comments     int
	moves        int
	moveError    error
	moveErrors   []error
	commentError error
}

func (f *fakeTrello) ValidateList(string, string) (string, error)     { return "Doing", nil }
func (f *fakeTrello) ListOpenCards(string) ([]scheduling.Card, error) { return f.cards, nil }
func (f *fakeTrello) DiscoverBoard(string) (trelloadapter.BoardDiscovery, error) {
	return trelloadapter.BoardDiscovery{Source: "fake", Lists: []trelloadapter.DiscoveredList{{ID: "peter", Name: "Peter"}, {ID: "shared", Name: "Peter & Liliia"}, {ID: "doing", Name: "Doing"}, {ID: "done", Name: "Done"}}}, nil
}
func (f *fakeTrello) ListOpenCardsInList(listID, listName string, _ trelloadapter.FieldMapping) ([]scheduling.Card, error) {
	var result []scheduling.Card
	for _, card := range f.cards {
		if card.ListID == listID || card.ListName == listName {
			result = append(result, card)
		}
	}
	return result, nil
}
func (f *fakeTrello) AddComment(string, string) error {
	f.comments++
	return f.commentError
}
func (f *fakeTrello) MoveCard(cardID, targetListID string) error {
	f.moves++
	err := f.moveError
	if len(f.moveErrors) > 0 {
		err = f.moveErrors[0]
		f.moveErrors = f.moveErrors[1:]
	}
	if err != nil {
		return err
	}
	for index := range f.cards {
		if f.cards[index].ID == cardID {
			f.cards[index].ListID = targetListID
			f.cards[index].ListName = "Doing"
		}
	}
	return nil
}

type fakeCalendar struct {
	events          []scheduling.Event
	existing        map[string]bool
	creates         int
	failCard        string
	accessError     error
	reschedules     map[string]scheduling.Assignment
	reconcileCreate bool
}

func (f *fakeCalendar) ListEvents(context.Context, time.Time, time.Time) ([]scheduling.Event, error) {
	return append([]scheduling.Event(nil), f.events...), nil
}
func (f *fakeCalendar) ListEventColors(context.Context) (map[string]bool, error) {
	return map[string]bool{}, nil
}
func (f *fakeCalendar) FindCard(_ context.Context, _ string, cardID string) (bool, error) {
	return f.existing[cardID], nil
}
func (f *fakeCalendar) FindCardEvent(_ context.Context, _ string, cardID string) (scheduling.Event, bool, error) {
	if !f.existing[cardID] {
		return scheduling.Event{}, false, nil
	}
	return scheduling.Event{ID: "event-" + cardID, Properties: map[string]string{"source": scheduling.Source}}, true, nil
}
func (f *fakeCalendar) RescheduleEvent(_ context.Context, eventID string, start, end time.Time) error {
	if f.reschedules == nil {
		f.reschedules = map[string]scheduling.Assignment{}
	}
	f.reschedules[eventID] = scheduling.Assignment{Start: start, End: end}
	return nil
}
func (f *fakeCalendar) CreateEvent(_ context.Context, _ string, assignment scheduling.Assignment, _, _, _ string) (string, bool, error) {
	f.creates++
	if assignment.Card.ID == f.failCard {
		return "", false, errors.New("injected failure")
	}
	f.existing[assignment.Card.ID] = true
	return "event-" + assignment.Card.ID, !f.reconcileCreate, nil
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
		Policy:  scheduling.SelectionPolicy{SourceListNames: []string{"Peter", "Peter & Liliia"}, ExcludeListNames: []string{"Doing", "Done"}, DoingListName: "Doing", DoneListName: "Done", PeterMemberID: "peter-member"},
	}
}

func TestReviewDoingMovesCompletedReschedulesIncompleteAndPlansRefill(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{
		{ID: "done-card", Name: "Finished", ListID: "doing", ListName: "Doing"},
		{ID: "doing-card", Name: "Continue", ListID: "doing", ListName: "Doing"},
		{ID: "next-card", Name: "Next", ListID: "peter", ListName: "Peter", Labels: readyLabels()},
	}}
	calendar := &fakeCalendar{existing: map[string]bool{"doing-card": true}}
	service := testService(t, trello, calendar)
	service.ListID = ""
	review, err := service.ReviewDoing(context.Background(), map[string]bool{"done-card": true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if review.RemainingCount != 1 || trello.moves != 1 || review.Items[0].Action != "move-to-done" {
		t.Fatalf("unexpected review result: %#v moves=%d", review, trello.moves)
	}
	if review.Items[1].Action != "rescheduled" || len(calendar.reschedules) != 1 {
		t.Fatalf("incomplete card was not rescheduled: %#v reschedules=%#v", review.Items[1], calendar.reschedules)
	}
	refill, err := service.PlanTopUp(context.Background(), DoingCapacity-review.RemainingCount, review.Plan.Assignments)
	if err != nil {
		t.Fatal(err)
	}
	if len(refill.Plan.Assignments) != 1 || refill.Plan.Assignments[0].Card.ID != "next-card" {
		t.Fatalf("unexpected refill plan: %#v", refill.Plan)
	}
}

func TestReviewDoingLabelsReconciledCreate(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "doing-card", Name: "Continue", ListID: "doing", ListName: "Doing"}}}
	calendar := &fakeCalendar{existing: map[string]bool{}, reconcileCreate: true}
	service := testService(t, trello, calendar)
	service.ListID = ""
	review, err := service.ReviewDoing(context.Background(), map[string]bool{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Items) != 1 || review.Items[0].Action != "reconciled-event" || review.Items[0].EventID != "event-doing-card" {
		t.Fatalf("unexpected review result: %#v", review)
	}
	if trello.moves != 0 || trello.comments != 0 {
		t.Fatalf("review reconciliation caused Trello side effects: moves=%d comments=%d", trello.moves, trello.comments)
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

func TestBoardAwarePlanSelectsAndDryRunDoesNotMove(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{
		{ID: "peter", Name: "Peter", ListID: "peter", ListName: "Peter", Labels: readyLabels()},
		{ID: "liliia", Name: "Liliia", ListID: "shared", ListName: "Peter & Liliia", Labels: readyLabels()},
		{ID: "manual", Name: "Manual", ListID: "peter", ListName: "Peter", Labels: []scheduling.Label{{Name: "P1"}, {Name: "T60"}}},
	}}
	calendar := &fakeCalendar{existing: map[string]bool{}}
	service := testService(t, trello, calendar)
	service.ListID = ""
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Plan.Assignments) != 1 || planned.Plan.Assignments[0].Card.ID != "peter" {
		t.Fatalf("unexpected plan: %#v", planned.Plan.Assignments)
	}
	if len(planned.Decisions) != 3 {
		t.Fatalf("missing decisions: %#v", planned.Decisions)
	}
	result := service.Execute(context.Background(), planned.Plan, true, false)
	if result.Results[0].MoveStatus != "would-move-to-Doing" || trello.moves != 0 || calendar.creates != 0 {
		t.Fatalf("dry-run wrote or missed intended move: result=%#v moves=%d creates=%d", result, trello.moves, calendar.creates)
	}
}

func TestLiveScheduleMovesOnlyAfterCalendarEventAndRetriesMoveOnExisting(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "a", Name: "A", ListID: "peter", ListName: "Peter", Labels: readyLabels()}}}
	calendar := &fakeCalendar{existing: map[string]bool{}}
	service := testService(t, trello, calendar)
	service.ListID = ""
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := service.Execute(context.Background(), planned.Plan, false, false)
	second := service.Execute(context.Background(), planned.Plan, false, false)
	if first.Created != 1 || first.Moved != 1 || first.Results[0].MoveStatus != "moved" {
		t.Fatalf("first run=%#v", first)
	}
	if second.Existing != 1 || second.Moved != 1 || calendar.creates != 1 || trello.moves != 2 {
		t.Fatalf("move retry/idempotency failed: first=%#v second=%#v creates=%d moves=%d", first, second, calendar.creates, trello.moves)
	}
}

func TestFullRerunRetriesMoveAfterCalendarSuccessWithoutDuplicateEvent(t *testing.T) {
	trello := &fakeTrello{
		cards:      []scheduling.Card{{ID: "a", Name: "A", ListID: "peter", ListName: "Peter", Labels: readyLabels()}},
		moveErrors: []error{errors.New("move failed")},
	}
	calendar := &fakeCalendar{existing: map[string]bool{}}
	service := testService(t, trello, calendar)
	service.ListID = ""
	firstPlan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	first := service.Execute(context.Background(), firstPlan.Plan, false, false)
	secondPlan, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second := service.Execute(context.Background(), secondPlan.Plan, false, false)
	if first.Created != 1 || first.Failed != 1 || first.Results[0].MoveStatus != "failed" {
		t.Fatalf("first run=%#v", first)
	}
	if len(secondPlan.Plan.Assignments) != 1 || secondPlan.Plan.Assignments[0].Card.ID != "a" {
		t.Fatalf("second plan did not retry source-list card: %#v", secondPlan.Plan.Assignments)
	}
	if second.Existing != 1 || second.Moved != 1 || calendar.creates != 1 || trello.moves != 2 {
		t.Fatalf("rerun failed: first=%#v second=%#v creates=%d moves=%d", first, second, calendar.creates, trello.moves)
	}
}

func TestBoardAwareAlreadyScheduledDoingCardIsSkipped(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "a", Name: "A", ListID: "doing", ListName: "Doing", Labels: readyLabels()}}}
	calendar := &fakeCalendar{existing: map[string]bool{"a": true}}
	service := testService(t, trello, calendar)
	service.ListID = ""
	service.Policy.SourceListNames = []string{"Doing"}
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Plan.Assignments) != 0 || len(planned.Decisions) != 1 || planned.Decisions[0].Reason != "excluded list" {
		t.Fatalf("unexpected scheduled Doing handling: plan=%#v decisions=%#v", planned.Plan.Assignments, planned.Decisions)
	}
}

func TestCalendarFailureDoesNotMoveAndMoveFailureIsPartial(t *testing.T) {
	trello := &fakeTrello{cards: []scheduling.Card{{ID: "a", Name: "A", ListID: "peter", ListName: "Peter", Labels: readyLabels()}, {ID: "b", Name: "B", ListID: "peter", ListName: "Peter", Labels: readyLabels(), Position: 1}}, moveError: errors.New("move failed")}
	calendar := &fakeCalendar{existing: map[string]bool{}, failCard: "a"}
	service := testService(t, trello, calendar)
	service.ListID = ""
	planned, err := service.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	result := service.Execute(context.Background(), planned.Plan, false, false)
	if trello.moves != 1 {
		t.Fatalf("calendar failure must not move failed event card; moves=%d result=%#v", trello.moves, result)
	}
	if result.Created != 1 || result.Failed != 2 || result.Results[1].Status != "event-created-move-failed" {
		t.Fatalf("unexpected partial move failure: %#v", result)
	}
}

func readyLabels() []scheduling.Label {
	return []scheduling.Label{{Name: "P1 High"}, {Name: "T60 1h"}, {Name: "AUTO"}}
}

func TestEventDescription(t *testing.T) {
	text := EventDescription(scheduling.Card{ID: "abc", URL: "https://trello.com/c/abc", Description: "Line one\nLine two", Priority: "High", EstimatedMinutes: 60})
	want := "Line one\nLine two\n\nCard: https://trello.com/c/abc\nTrello card ID: abc\nPriority: High\nDuration: 60 minutes"
	if text != want {
		t.Fatalf("description:\n%s\nwant:\n%s", text, want)
	}
}
