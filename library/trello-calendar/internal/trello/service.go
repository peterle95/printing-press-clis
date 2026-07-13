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

type cardResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	URL         string             `json:"url"`
	Desc        string             `json:"desc"`
	Due         *string            `json:"due"`
	DueComplete bool               `json:"dueComplete"`
	Labels      []scheduling.Label `json:"labels"`
	Position    float64            `json:"pos"`
	Closed      bool               `json:"closed"`
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

func (s *Service) ListOpenCards(listID string) ([]scheduling.Card, error) {
	params := map[string]string{
		"filter": "open",
		"fields": "id,name,url,due,dueComplete,labels,pos,closed",
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
		DueComplete: card.DueComplete,
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
	}
	return result, nil
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
