// Copyright 2026 peter-moelzer. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"time"
	"trello-calendar-pp-cli/internal/config"
	"trello-calendar-pp-cli/internal/googlecalendar"
	"trello-calendar-pp-cli/internal/scheduling"
	"trello-calendar-pp-cli/internal/trello"
	"trello-calendar-pp-cli/internal/workflow"

	"github.com/spf13/cobra"
)

type plannerCLIOptions struct {
	duration        int
	dayStart        string
	dayEnd          string
	preferredTime   string
	includeWeekends bool
	maxEvents       int
	titlePrefix     string
	commentOnCard   bool
	reviewDoing     bool
	skipReviewDoing bool
}

func defaultPlannerCLIOptions() plannerCLIOptions {
	return plannerCLIOptions{
		duration: config.DefaultDuration, dayStart: config.DefaultDayStart, dayEnd: config.DefaultDayEnd,
		preferredTime: config.DefaultPreferred, maxEvents: config.DefaultMaxEvents,
	}
}

func newPreviewCmd(flags *rootFlags) *cobra.Command {
	opts := defaultPlannerCLIOptions()
	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Show next week's proposed Trello card schedule",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			service, _, err := newWorkflowService(cmd, flags, false, &opts)
			if err != nil {
				return err
			}
			planned, err := service.Plan(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			return printPlan(cmd, flags, planned)
		},
	}
	addPlannerFlags(cmd, &opts)
	return cmd
}

func newScheduleCmd(flags *rootFlags) *cobra.Command {
	opts := defaultPlannerCLIOptions()
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Create the confirmed next-week schedule in Google Calendar",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			service, _, err := newWorkflowService(cmd, flags, !flags.dryRun, &opts)
			if err != nil {
				return err
			}
			if opts.reviewDoing && !opts.skipReviewDoing && strings.TrimSpace(service.ListID) == "" {
				return runDoingReviewWorkflow(cmd, flags, service)
			}
			planned, err := service.Plan(cmd.Context())
			if err != nil {
				return apiErr(err)
			}
			if !flags.asJSON {
				if err := printPlan(cmd, flags, planned); err != nil {
					return err
				}
			}
			if !flags.dryRun && !flags.yes {
				if flags.noInput || flags.asJSON {
					return usageErr(fmt.Errorf("live scheduling in non-interactive or JSON mode requires --yes"))
				}
				confirmed, err := confirm(cmd.InOrStdin(), cmd.ErrOrStderr())
				if err != nil {
					return err
				}
				if !confirmed {
					fmt.Fprintln(cmd.ErrOrStderr(), "Scheduling cancelled.")
					return nil
				}
			}
			execution := service.Execute(cmd.Context(), planned.Plan, flags.dryRun, opts.commentOnCard)
			if flags.asJSON {
				if err := flags.printJSON(cmd, map[string]any{"command": "schedule", "plan": planned, "execution": execution}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\nCreated: %d  Existing: %d  Failed: %d\n", execution.Created, execution.Existing, execution.Failed)
				for _, item := range execution.Results {
					if item.Error != "" {
						fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", item.CardName, item.Error)
					}
				}
			}
			if execution.Failed > 0 {
				return fmt.Errorf("schedule completed with %d failed assignment(s)", execution.Failed)
			}
			return nil
		},
	}
	addPlannerFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.commentOnCard, "comment-on-card", false, "Add a scheduling comment to each Trello card")
	cmd.Flags().BoolVar(&opts.reviewDoing, "review-doing", true, "Review Doing cards and refill the list to four cards")
	cmd.Flags().BoolVar(&opts.skipReviewDoing, "skip-doing-review", false, "Skip the Doing review phase")
	return cmd
}

// PATCH: Expose the Doing review as a dedicated command and as the default
// board-aware schedule phase. Legacy trello_list_id mode remains unchanged.
func newReviewCmd(flags *rootFlags) *cobra.Command {
	opts := defaultPlannerCLIOptions()
	cmd := &cobra.Command{
		Use:   "review",
		Short: "Review Doing cards, reschedule them, and refill Doing to four cards",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			service, _, err := newWorkflowService(cmd, flags, !flags.dryRun, &opts)
			if err != nil {
				return err
			}
			return runDoingReviewWorkflow(cmd, flags, service)
		},
	}
	addPlannerFlags(cmd, &opts)
	return cmd
}

func runDoingReviewWorkflow(cmd *cobra.Command, flags *rootFlags, service *workflow.Service) error {
	ctx := cmd.Context()
	initial, err := service.ReviewDoing(ctx, map[string]bool{}, true)
	if err != nil {
		return apiErr(err)
	}
	completed := map[string]bool{}
	for _, item := range initial.Items {
		// PATCH: Only ask about Doing cards that already have a Calendar event;
		// unscheduled Doing cards are proposed directly in the weekly preview.
		if item.EventID == "" || flags.noInput || flags.yes {
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Was already-scheduled Doing card %q completed? [y/N] (No = retry/reschedule) ", item.Card.Name)
		answer, readErr := confirm(cmd.InOrStdin(), cmd.ErrOrStderr())
		if readErr != nil {
			return readErr
		}
		if answer {
			completed[item.Card.ID] = true
		}
	}
	preview, err := service.ReviewDoing(ctx, completed, true)
	if err != nil {
		return apiErr(err)
	}
	capacity := workflow.DoingCapacity - preview.RemainingCount
	refillPreview, err := service.PlanTopUp(ctx, capacity, preview.Plan.Assignments)
	if err != nil {
		return apiErr(err)
	}
	if !flags.asJSON {
		fmt.Fprintln(cmd.OutOrStdout(), "\nProposed Doing review:")
		for _, item := range preview.Items {
			if item.Error != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s (%s)\n", item.Card.Name, item.Action, item.Error)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "- %s: %s\n", item.Card.Name, item.Action)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "\nProposed week for Doing cards:")
		if err := printPlan(cmd, flags, workflow.PlanResult{ListName: "Doing", Plan: preview.Plan}); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Proposed refill from source lists:")
		if err := printPlan(cmd, flags, refillPreview); err != nil {
			return err
		}
	}
	if !flags.dryRun && !flags.yes {
		if flags.noInput || flags.asJSON {
			return usageErr(fmt.Errorf("live Doing review in non-interactive or JSON mode requires --yes"))
		}
		confirmed, confirmErr := confirm(cmd.InOrStdin(), cmd.ErrOrStderr())
		if confirmErr != nil {
			return confirmErr
		}
		if !confirmed {
			fmt.Fprintln(cmd.ErrOrStderr(), "Doing review cancelled.")
			return nil
		}
	}

	review := preview
	refill := refillPreview
	execution := workflow.ExecutionResult{DryRun: flags.dryRun, Planned: len(refill.Plan.Assignments), Results: []workflow.AssignmentResult{}}
	if !flags.dryRun {
		review, err = service.ReviewDoing(ctx, completed, false)
		if err != nil {
			return apiErr(err)
		}
		refill, err = service.PlanTopUp(ctx, workflow.DoingCapacity-review.RemainingCount, nil)
		if err != nil {
			return apiErr(err)
		}
		execution = service.Execute(ctx, refill.Plan, false, false)
	} else {
		execution = service.Execute(ctx, refill.Plan, true, false)
	}

	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{"command": "review", "review": review, "refill": refill, "execution": execution})
	}
	for _, item := range review.Items {
		if item.Error != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s (%s)\n", item.Card.Name, item.Action, item.Error)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", item.Card.Name, item.Action)
		}
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Doing remaining: %d/%d; refill assignments: %d\n", review.RemainingCount, workflow.DoingCapacity, len(refill.Plan.Assignments))
	if execution.Failed > 0 {
		return fmt.Errorf("Doing review completed with %d failed assignment(s)", execution.Failed)
	}
	if !flags.dryRun {
		failed := 0
		for _, item := range review.Items {
			if item.Action == "failed" || item.Action == "unscheduled" {
				failed++
			}
		}
		if failed > 0 {
			return fmt.Errorf("Doing review completed with %d failed card action(s)", failed)
		}
	}
	return nil
}

func addPlannerFlags(cmd *cobra.Command, opts *plannerCLIOptions) {
	cmd.Flags().IntVar(&opts.duration, "duration", config.DefaultDuration, "Event duration in minutes")
	cmd.Flags().StringVar(&opts.dayStart, "day-start", config.DefaultDayStart, "Scheduling window start (HH:MM)")
	cmd.Flags().StringVar(&opts.dayEnd, "day-end", config.DefaultDayEnd, "Scheduling window end (HH:MM)")
	cmd.Flags().StringVar(&opts.preferredTime, "preferred-time", config.DefaultPreferred, "Preferred start time (HH:MM)")
	cmd.Flags().BoolVar(&opts.includeWeekends, "include-weekends", false, "Include Saturday and Sunday")
	cmd.Flags().IntVar(&opts.maxEvents, "max-events-per-day", config.DefaultMaxEvents, "Maximum existing events allowed per day")
	cmd.Flags().StringVar(&opts.titlePrefix, "title-prefix", "", "Calendar event title prefix")
}

func newWorkflowService(cmd *cobra.Command, flags *rootFlags, persistRefresh bool, cliOpts *plannerCLIOptions) (*workflow.Service, *config.Config, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, nil, configErr(err)
	}
	if err := cfg.ValidatePlanner(); err != nil {
		return nil, nil, configErr(err)
	}
	if cliOpts != nil {
		applyPlannerOverrides(cmd, cfg, cliOpts)
		if err := cfg.Validate(); err != nil {
			return nil, nil, usageErr(err)
		}
	}
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, nil, configErr(err)
	}
	trelloClient, err := flags.newClient()
	if err != nil {
		return nil, nil, err
	}
	// Planner dry-runs still require live GETs; mutations are gated by the executor.
	trelloClient.DryRun = false
	trelloClient.NoCache = true
	store := googlecalendar.TokenStore{Path: cfg.TokenPath()}
	httpClient, err := googlecalendar.NewHTTPClient(cmd.Context(), cfg, store, persistRefresh)
	if err != nil {
		return nil, nil, authErr(err)
	}
	calendar := googlecalendar.NewClient(cfg.GoogleBaseURL, cfg.GoogleCalendarID, location, httpClient)
	calendar.SetRateLimit(flags.rateLimit)
	if flags.verbose {
		fmt.Fprintf(cmd.ErrOrStderr(), "planner timezone=%s calendar=%s list=%s\n", cfg.Timezone, cfg.GoogleCalendarID, cfg.TrelloListID)
	}
	// PATCH: Wire board-aware defaults while preserving trello_list_id legacy mode.
	return &workflow.Service{
		Trello: trello.New(trelloClient), Calendar: calendar, Now: time.Now,
		BoardID: cfg.TrelloBoardID, ListID: cfg.TrelloListID,
		Policy: scheduling.SelectionPolicy{
			SourceListNames: cfg.SourceListNames, ExcludeListNames: cfg.ExcludeListNames, DoingListName: cfg.DoingListName,
			DoneListName:  cfg.DoneListName,
			PeterMemberID: cfg.PeterMemberID, AllowLiliiaCards: cfg.AllowLiliiaCards,
		},
		Options: scheduling.Options{
			Location: location, DurationMinutes: cfg.DurationMinutes, PreferredTime: cfg.PreferredTime,
			DayStart: cfg.DayStart, DayEnd: cfg.DayEnd, MaxEventsPerDay: cfg.MaxEventsPerDay,
			IncludeWeekends: cfg.IncludeWeekends, TitlePrefix: cfg.TitlePrefix, PriorityColors: cfg.PriorityColors(),
		},
	}, cfg, nil
}

func applyPlannerOverrides(cmd *cobra.Command, cfg *config.Config, opts *plannerCLIOptions) {
	if cmd.Flags().Changed("duration") {
		cfg.DurationMinutes = opts.duration
	}
	if cmd.Flags().Changed("day-start") {
		cfg.DayStart = opts.dayStart
	}
	if cmd.Flags().Changed("day-end") {
		cfg.DayEnd = opts.dayEnd
	}
	if cmd.Flags().Changed("preferred-time") {
		cfg.PreferredTime = opts.preferredTime
	}
	if cmd.Flags().Changed("include-weekends") {
		cfg.IncludeWeekends = opts.includeWeekends
	}
	if cmd.Flags().Changed("max-events-per-day") {
		cfg.MaxEventsPerDay = opts.maxEvents
	}
	if cmd.Flags().Changed("title-prefix") {
		cfg.TitlePrefix = opts.titlePrefix
	}
}

func printPlan(cmd *cobra.Command, flags *rootFlags, planned workflow.PlanResult) error {
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{"command": "preview", "result": planned})
	}
	for _, warning := range planned.Warnings {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", warning)
	}
	for _, day := range planned.Plan.Days {
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", day.Weekday, day.Date)
		fmt.Fprintf(cmd.OutOrStdout(), "Existing events: %d\n", day.ExistingEvents)
		if day.Assignment != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Proposed card: %s\n", day.Assignment.Card.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "Time: %s–%s\n\n", day.Assignment.Start.Format("15:04"), day.Assignment.End.Format("15:04"))
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Skipped: %s\n\n", day.Skipped)
		}
	}
	return nil
}

func confirm(in io.Reader, out io.Writer) (bool, error) {
	fmt.Fprint(out, "Create these Google Calendar events? [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
