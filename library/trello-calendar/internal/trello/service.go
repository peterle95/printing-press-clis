// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

// Package trello adapts the generated Trello client to planner domain types.
// PATCH: Add typed, validated Trello workflow calls without coupling scheduling logic to HTTP.
package trello

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"trello-calendar-pp-cli/internal/client"
	"trello-calendar-pp-cli/internal/scheduling"
)

type Service struct {
	client *client.Client
}

func New(c *client.Client) *Service { return &Service{client: c} }

type listResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BoardID string `json:"idBoard"`
	Closed  bool   `json:"closed"`
}

type BoardDiscovery struct {
	Source   string            `json:"source"`
	Lists    []DiscoveredList  `json:"lists"`
	Fields   FieldMapping      `json:"fields"`
	Labels   map[string]string `json:"labels,omitempty"`
	Warnings []string          `json:"warnings,omitempty"`
}

type DiscoveredList struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Closed bool   `json:"closed"`
}

type FieldMapping struct {
	Priority   string            `json:"priority,omitempty"`
	Time       string            `json:"time,omitempty"`
	Automation string            `json:"automation,omitempty"`
	Status     string            `json:"status,omitempty"`
	Options    map[string]string `json:"options,omitempty"`
}

type customFieldResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Options []struct {
		ID    string `json:"id"`
		Value struct {
			Text string `json:"text"`
		} `json:"value"`
	} `json:"options"`
}

type labelResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type cardResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	URL         string             `json:"url"`
	Desc        string             `json:"desc"`
	ListID      string             `json:"idList"`
	MemberIDs   []string           `json:"idMembers"`
	Due         *string            `json:"due"`
	DueComplete bool               `json:"dueComplete"`
	Labels      []scheduling.Label `json:"labels"`
	Position    float64            `json:"pos"`
	Closed      bool               `json:"closed"`
	// PATCH: Trello exposes one dateLastActivity field; creation ordering is
	// derived from the card ID, so avoid a duplicate JSON tag during vet.
	CreatedAt  *string `json:"-"`
	ActivityAt *string `json:"dateLastActivity"`
}

type customFieldItemResponse struct {
	IDCustomField string `json:"idCustomField"`
	IDValue       string `json:"idValue"`
	Value         struct {
		Text   string `json:"text"`
		Number string `json:"number"`
	} `json:"value"`
}

type pluginDataResponse struct {
	Value string `json:"value"`
}

type actionResponse struct {
	Data struct {
		Text string `json:"text"`
	} `json:"data"`
}

func (s *Service) ValidateList(boardID, listID string) (string, error) {
	// pp:client-call -- this adapter invokes the generated live Trello client.
	raw, err := s.client.Get("/lists/"+url.PathEscape(listID), map[string]string{"fields": "id,name,idBoard,closed"})
	if err != nil {
		return "", fmt.Errorf("read Trello list: %w", err)
	}
	var list listResponse
	if err := json.Unmarshal(raw, &list); err != nil {
		return "", fmt.Errorf("decode Trello list response: %w", err)
	}
	if list.ID == "" || list.BoardID == "" || list.Name == "" {
		return "", fmt.Errorf("malformed Trello list response")
	}
	if list.Closed {
		return "", fmt.Errorf("configured Trello list is archived")
	}
	if list.BoardID != boardID {
		return "", fmt.Errorf("configured Trello list belongs to board %s, not %s", list.BoardID, boardID)
	}
	if _, err := s.client.Get("/boards/"+url.PathEscape(boardID), map[string]string{"fields": "id,name,closed"}); err != nil {
		return "", fmt.Errorf("read Trello board: %w", err)
	}
	return list.Name, nil
}

// PATCH: Discover board list IDs and custom field option IDs by name at runtime.
func (s *Service) DiscoverBoard(boardID string) (BoardDiscovery, error) {
	discovery := BoardDiscovery{Source: "trello-api", Fields: FieldMapping{Options: map[string]string{}}, Labels: map[string]string{}}
	raw, err := s.client.Get("/boards/"+url.PathEscape(boardID)+"/lists", map[string]string{"filter": "open", "fields": "id,name,closed"})
	if err != nil {
		return discovery, fmt.Errorf("discover Trello board lists: %w", err)
	}
	var lists []listResponse
	if err := json.Unmarshal(raw, &lists); err != nil {
		return discovery, fmt.Errorf("decode Trello board lists: %w", err)
	}
	for _, list := range lists {
		if list.ID != "" && list.Name != "" {
			discovery.Lists = append(discovery.Lists, DiscoveredList{ID: list.ID, Name: list.Name, Closed: list.Closed})
		}
	}
	labelsRaw, err := s.client.Get("/boards/"+url.PathEscape(boardID)+"/labels", map[string]string{"fields": "id,name,color", "limit": "1000"})
	if err != nil {
		discovery.Warnings = append(discovery.Warnings, "label discovery unavailable: "+err.Error())
	} else {
		var labels []labelResponse
		if err := json.Unmarshal(labelsRaw, &labels); err != nil {
			discovery.Warnings = append(discovery.Warnings, "label discovery decode failed: "+err.Error())
		} else {
			for _, label := range labels {
				if label.ID != "" && strings.TrimSpace(label.Name) != "" {
					discovery.Labels[strings.TrimSpace(label.Name)] = label.ID
				}
			}
		}
	}
	fieldsRaw, err := s.client.Get("/boards/"+url.PathEscape(boardID)+"/customFields", nil)
	if err != nil {
		discovery.Warnings = append(discovery.Warnings, "custom field discovery unavailable: "+err.Error())
		return discovery, nil
	}
	var fields []customFieldResponse
	if err := json.Unmarshal(fieldsRaw, &fields); err != nil {
		discovery.Warnings = append(discovery.Warnings, "custom field discovery decode failed: "+err.Error())
		return discovery, nil
	}
	for _, field := range fields {
		switch normalizeName(field.Name) {
		case "priority":
			discovery.Fields.Priority = field.ID
		case "time":
			discovery.Fields.Time = field.ID
		case "automation":
			discovery.Fields.Automation = field.ID
		case "status":
			discovery.Fields.Status = field.ID
		}
		for _, option := range field.Options {
			if option.ID != "" && option.Value.Text != "" {
				discovery.Fields.Options[option.ID] = option.Value.Text
			}
		}
	}
	return discovery, nil
}

func (s *Service) ListOpenCardsInList(listID, listName string, fields FieldMapping) ([]scheduling.Card, error) {
	cards, err := s.listCards(listID, "open", listName)
	if err != nil {
		return nil, err
	}
	for index := range cards {
		s.enrichCard(&cards[index], fields)
	}
	return cards, nil
}

func (s *Service) ListOpenCards(listID string) ([]scheduling.Card, error) {
	return s.listCards(listID, "open", "")
}

func (s *Service) listCards(listID, filter, listName string) ([]scheduling.Card, error) {
	params := map[string]string{
		"filter": filter,
		"fields": "id,name,url,desc,idList,idMembers,due,dueComplete,labels,pos,closed,dateLastActivity",
		"limit":  "1000",
	}
	seen := map[string]bool{}
	var result []scheduling.Card
	for page := 0; page < 100; page++ {
		// pp:client-call -- pagination is executed against Trello, never synthesized.
		raw, err := s.client.Get("/lists/"+url.PathEscape(listID)+"/cards", params)
		if err != nil {
			return nil, fmt.Errorf("list Trello cards: %w", err)
		}
		var cards []cardResponse
		if err := json.Unmarshal(raw, &cards); err != nil {
			return nil, fmt.Errorf("decode Trello cards response: %w", err)
		}
		for _, card := range cards {
			if card.ID == "" || strings.TrimSpace(card.Name) == "" {
				return nil, fmt.Errorf("malformed Trello card response")
			}
			if seen[card.ID] {
				continue
			}
			seen[card.ID] = true
			domain, err := toCard(card)
			if err != nil {
				return nil, err
			}
			domain.ListName = listName
			if !domain.Closed {
				result = append(result, domain)
			}
		}
		if len(cards) < 1000 {
			break
		}
		params["before"] = cards[len(cards)-1].ID
	}
	return result, nil
}

func toCard(card cardResponse) (scheduling.Card, error) {
	result := scheduling.Card{
		ID:          card.ID,
		Name:        card.Name,
		URL:         card.URL,
		Description: card.Desc,
		ListID:      card.ListID,
		MemberIDs:   card.MemberIDs,
		DueComplete: card.DueComplete,
		Completed:   card.DueComplete,
		Labels:      card.Labels,
		Position:    card.Position,
		Closed:      card.Closed,
	}
	if card.Due != nil && strings.TrimSpace(*card.Due) != "" {
		due, err := timeParse(*card.Due)
		if err != nil {
			return scheduling.Card{}, fmt.Errorf("card %s has malformed due date: %w", card.ID, err)
		}
		result.Due = &due
		result.DueDate = &due
	}
	if card.ActivityAt != nil && strings.TrimSpace(*card.ActivityAt) != "" {
		activity, err := timeParse(*card.ActivityAt)
		if err == nil {
			result.LastActivityAt = &activity
		}
	}
	if created := createdAtFromTrelloID(card.ID); !created.IsZero() {
		result.CreatedAt = &created
	}
	return result, nil
}

func (s *Service) enrichCard(card *scheduling.Card, fields FieldMapping) {
	raw, err := s.client.Get("/cards/"+url.PathEscape(card.ID)+"/customFieldItems", nil)
	if err != nil {
		card.FieldWarnings = append(card.FieldWarnings, "customFieldItems unavailable: "+err.Error())
	} else {
		var items []customFieldItemResponse
		if err := json.Unmarshal(raw, &items); err != nil {
			card.FieldWarnings = append(card.FieldWarnings, "customFieldItems decode failed: "+err.Error())
		} else {
			applyCustomFields(card, fields, items)
		}
	}
	raw, err = s.client.Get("/cards/"+url.PathEscape(card.ID)+"/pluginData", nil)
	if err != nil {
		card.FieldWarnings = append(card.FieldWarnings, "pluginData unavailable: "+err.Error())
		return
	}
	var pluginData []pluginDataResponse
	if err := json.Unmarshal(raw, &pluginData); err != nil {
		card.FieldWarnings = append(card.FieldWarnings, "pluginData decode failed: "+err.Error())
		return
	}
	applyPluginData(card, pluginData)
}

func applyCustomFields(card *scheduling.Card, fields FieldMapping, items []customFieldItemResponse) {
	for _, item := range items {
		value := item.Value.Text
		if value == "" && item.IDValue != "" {
			value = fields.Options[item.IDValue]
		}
		if value == "" {
			value = item.Value.Number
		}
		switch item.IDCustomField {
		case fields.Priority:
			card.Priority = strings.TrimSpace(value)
		case fields.Time:
			card.EstimatedMinutes = parseMinutes(value)
		case fields.Automation:
			card.Automation = strings.TrimSpace(value)
		case fields.Status:
			card.Status = strings.TrimSpace(value)
		}
	}
}

func applyPluginData(card *scheduling.Card, data []pluginDataResponse) {
	for _, entry := range data {
		var payload any
		if json.Unmarshal([]byte(entry.Value), &payload) != nil {
			continue
		}
		walkFields(payload, func(key, value string) {
			switch normalizeName(key) {
			case "priority":
				if card.Priority == "" {
					card.Priority = value
				}
			case "time":
				if card.EstimatedMinutes == 0 {
					card.EstimatedMinutes = parseMinutes(value)
				}
			case "automation":
				if card.Automation == "" {
					card.Automation = value
				}
			case "status":
				if card.Status == "" {
					card.Status = value
				}
			}
		})
	}
}

func walkFields(value any, visit func(string, string)) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch v := child.(type) {
			case string:
				visit(key, strings.TrimSpace(v))
			case float64:
				visit(key, fmt.Sprintf("%.0f", v))
			default:
				walkFields(v, visit)
			}
		}
	case []any:
		for _, child := range typed {
			walkFields(child, visit)
		}
	}
}

func parseMinutes(value string) int {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(lower, "%d", &n); err == nil && n > 0 {
		if strings.Contains(lower, "h") && !strings.Contains(lower, "m") {
			return n * 60
		}
		return n
	}
	return 0
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func createdAtFromTrelloID(id string) time.Time {
	if len(id) < 8 {
		return time.Time{}
	}
	var seconds int64
	if _, err := fmt.Sscanf(id[:8], "%x", &seconds); err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0).UTC()
}

var timeParse = func(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }

func (s *Service) HasComment(cardID, text string) (bool, error) {
	raw, err := s.client.Get("/cards/"+url.PathEscape(cardID)+"/actions", map[string]string{"filter": "commentCard", "limit": "1000"})
	if err != nil {
		return false, fmt.Errorf("list Trello comments: %w", err)
	}
	var actions []actionResponse
	if err := json.Unmarshal(raw, &actions); err != nil {
		return false, fmt.Errorf("decode Trello comments response: %w", err)
	}
	for _, action := range actions {
		if action.Data.Text == text {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) AddComment(cardID, text string) error {
	if exists, err := s.HasComment(cardID, text); err != nil {
		return err
	} else if exists {
		return nil
	}
	_, _, err := s.client.PostWithParams("/cards/"+url.PathEscape(cardID)+"/actions/comments", map[string]string{"text": text}, nil)
	if err != nil {
		if exists, checkErr := s.HasComment(cardID, text); checkErr == nil && exists {
			return nil
		}
		return fmt.Errorf("add Trello comment: %w", err)
	}
	return nil
}

func (s *Service) GetCard(cardID string) (*scheduling.Card, error) {
	raw, err := s.client.Get("/cards/"+url.PathEscape(cardID), map[string]string{"fields": "id,name,url,due,dueComplete,labels,pos,closed,desc"})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return nil, nil
		}
		return nil, fmt.Errorf("get Trello card: %w", err)
	}
	var card cardResponse
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("decode Trello card response: %w", err)
	}
	domain, err := toCard(card)
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (s *Service) CreateCard(listID, name, desc string) (*scheduling.Card, error) {
	params := map[string]string{
		"idList": listID,
		"name":   name,
	}
	if desc != "" {
		params["desc"] = desc
	}
	raw, _, err := s.client.PostWithParams("/cards", params, nil)
	if err != nil {
		return nil, fmt.Errorf("create Trello card: %w", err)
	}
	var card cardResponse
	if err := json.Unmarshal(raw, &card); err != nil {
		return nil, fmt.Errorf("decode Trello card response: %w", err)
	}
	if card.ID == "" || strings.TrimSpace(card.Name) == "" {
		return nil, fmt.Errorf("create Trello card: malformed response (empty id or name)")
	}
	domain, err := toCard(card)
	if err != nil {
		return nil, err
	}
	return &domain, nil
}

func (s *Service) ArchiveCard(cardID string) error {
	_, _, err := s.client.PutWithParams("/cards/"+url.PathEscape(cardID), map[string]string{"closed": "true"}, nil)
	if err != nil {
		return fmt.Errorf("archive Trello card: %w", err)
	}
	return nil
}

func (s *Service) MoveCard(cardID, targetListID string) error {
	_, _, err := s.client.PutWithParams("/cards/"+url.PathEscape(cardID), map[string]string{"idList": targetListID}, nil)
	if err != nil {
		return fmt.Errorf("move Trello card: %w", err)
	}
	return nil
}

func (s *Service) ListCards(listID string, filter string) ([]scheduling.Card, error) {
	params := map[string]string{
		"filter": filter,
		"fields": "id,name,url,due,dueComplete,labels,pos,closed",
		"limit":  "1000",
	}
	seen := map[string]bool{}
	var result []scheduling.Card
	for page := 0; page < 100; page++ {
		raw, err := s.client.Get("/lists/"+url.PathEscape(listID)+"/cards", params)
		if err != nil {
			return nil, fmt.Errorf("list Trello cards: %w", err)
		}
		var cards []cardResponse
		if err := json.Unmarshal(raw, &cards); err != nil {
			return nil, fmt.Errorf("decode Trello cards response: %w", err)
		}
		for _, card := range cards {
			if card.ID == "" || strings.TrimSpace(card.Name) == "" {
				return nil, fmt.Errorf("malformed Trello card response")
			}
			if seen[card.ID] {
				continue
			}
			seen[card.ID] = true
			domain, err := toCard(card)
			if err != nil {
				return nil, err
			}
			switch filter {
			case "closed":
				if domain.Closed {
					result = append(result, domain)
				}
			case "open":
				if !domain.Closed {
					result = append(result, domain)
				}
			default:
				result = append(result, domain)
			}
		}
		if len(cards) < 1000 {
			break
		}
		params["before"] = cards[len(cards)-1].ID
	}
	return result, nil
}
