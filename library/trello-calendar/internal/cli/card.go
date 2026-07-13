// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.
// PATCH: Hand-written high-level card operations (get, create, archive, move).

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"
	"unicode/utf8"
	"trello-calendar-pp-cli/internal/client"
	"trello-calendar-pp-cli/internal/trello"

	"github.com/spf13/cobra"
)

// cardDetail matches the Trello API card response including the description
// field that scheduling.Card drops.
type cardDetail struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	URL         string             `json:"url"`
	Desc        string             `json:"desc"`
	Due         *string            `json:"due"`
	DueComplete bool               `json:"dueComplete"`
	Labels      []json.RawMessage  `json:"labels"`
	Pos         float64            `json:"pos"`
	Closed      bool               `json:"closed"`
}

func newCardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "card",
		Short: "Manage Trello cards (get, create, archive, move)",
	}
	cmd.AddCommand(newCardGetCmd(flags))
	cmd.AddCommand(newCardCreateCmd(flags))
	cmd.AddCommand(newCardArchiveCmd(flags))
	cmd.AddCommand(newCardMoveCmd(flags))
	return cmd
}

// PATCH: Uses raw client instead of trello.Service.GetCard to preserve the
// Desc field which scheduling.Card drops. Keep in sync with service.go.
// card get <card-id>
func newCardGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <card-id>",
		Short:       "Get detailed info about a single Trello card",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would fetch card %s\n", args[0])
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get("/cards/"+url.PathEscape(args[0]), map[string]string{
				"fields": "id,name,url,due,dueComplete,labels,pos,closed,desc",
			})
			if err != nil {
				var apiErrObj *client.APIError
				if As(err, &apiErrObj) && apiErrObj.StatusCode == 404 {
					return notFoundErr(fmt.Errorf("card %q not found", args[0]))
				}
				return classifyAPIError(err, flags)
			}
			var card cardDetail
			if err := json.Unmarshal(raw, &card); err != nil {
				return apiErr(fmt.Errorf("decode card response: %w", err))
			}
			if card.ID == "" {
				return notFoundErr(fmt.Errorf("card %q not found", args[0]))
			}
			if flags.asJSON {
				return flags.printJSON(cmd, card)
			}
			printCard(cmd, &card)
			return nil
		},
	}
	return cmd
}

// card create --name --list-id [--desc]
func newCardCreateCmd(flags *rootFlags) *cobra.Command {
	var name, listID, desc string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Trello card in a list",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate required flags before anything (even before dry-run)
			if name == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}
			if listID == "" {
				return usageErr(fmt.Errorf("--list-id is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would create card %q in list %s\n", name, listID)
				return nil
			}
			// Confirmation prompt for live mutation
			if !flags.yes {
				if flags.noInput || flags.asJSON {
					return usageErr(fmt.Errorf("live mutation requires --yes in non-interactive or JSON mode"))
				}
				confirmed, err := confirmCardAction(cmd.InOrStdin(), cmd.ErrOrStderr(), "create this card")
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.ErrOrStderr(), "Create cancelled.")
					return nil
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			service := trello.New(c)
			card, err := service.CreateCard(listID, name, desc)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, card)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created card %q (id: %s) in list %s\n", card.Name, card.ID, listID)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Card name (required)")
	cmd.Flags().StringVar(&listID, "list-id", "", "Target list ID (required)")
	cmd.Flags().StringVar(&desc, "desc", "", "Card description (optional)")
	return cmd
}

// card archive <card-id>
func newCardArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <card-id>",
		Short: "Archive (close) a Trello card",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would archive card %s\n", args[0])
				return nil
			}
			if !flags.yes {
				if flags.noInput || flags.asJSON {
					return usageErr(fmt.Errorf("live mutation requires --yes in non-interactive or JSON mode"))
				}
				confirmed, err := confirmCardAction(cmd.InOrStdin(), cmd.ErrOrStderr(), "archive this card")
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.ErrOrStderr(), "Archive cancelled.")
					return nil
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			service := trello.New(c)
			if err := service.ArchiveCard(args[0]); err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"command": "archive", "card_id": args[0], "status": "archived"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived card %s\n", args[0])
			return nil
		},
	}
	return cmd
}

// card move <card-id> --list-id <list-id>
func newCardMoveCmd(flags *rootFlags) *cobra.Command {
	var listID string
	cmd := &cobra.Command{
		Use:   "move <card-id>",
		Short: "Move a Trello card to another list",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if listID == "" {
				return usageErr(fmt.Errorf("--list-id is required"))
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "dry-run: would move card %s to list %s\n", args[0], listID)
				return nil
			}
			if !flags.yes {
				if flags.noInput || flags.asJSON {
					return usageErr(fmt.Errorf("live mutation requires --yes in non-interactive or JSON mode"))
				}
				confirmed, err := confirmCardAction(cmd.InOrStdin(), cmd.ErrOrStderr(), "move this card to another list")
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.ErrOrStderr(), "Move cancelled.")
					return nil
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			service := trello.New(c)
			if err := service.MoveCard(args[0], listID); err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"command": "move", "card_id": args[0], "target_list_id": listID, "status": "moved"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Moved card %s to list %s\n", args[0], listID)
			return nil
		},
	}
	cmd.Flags().StringVar(&listID, "list-id", "", "Target list ID (required)")
	return cmd
}

func printCard(cmd *cobra.Command, card *cardDetail) {
	status := "open"
	if card.Closed {
		status = "archived"
	}
	due := ""
	if card.Due != nil && strings.TrimSpace(*card.Due) != "" {
		// Trello returns ISO 8601; show date portion only.
		due = *card.Due
		if len(due) >= 10 {
			due = due[:10]
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Card: %s\n", card.Name)
	fmt.Fprintf(cmd.OutOrStdout(), "ID: %s\n", card.ID)
	fmt.Fprintf(cmd.OutOrStdout(), "URL: %s\n", card.URL)
	fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", status)
	if due != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "Due: %s\n", due)
	}
	if card.Desc != "" {
		desc := card.Desc
		if utf8.RuneCountInString(desc) > 60 {
			runes := []rune(desc)
			desc = string(runes[:57]) + "..."
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Description: %s\n", desc)
	}
}

func confirmCardAction(in io.Reader, out io.Writer, action string) (bool, error) {
	fmt.Fprint(out, fmt.Sprintf("Are you sure you want to %s? [y/N] ", action))
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
