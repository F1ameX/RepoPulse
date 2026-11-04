package domain

import "time"

const ActivityHistoryScope = "activity_history"

// ActivityCommit belongs to the history reachable from the pinned default SHA.
type ActivityCommit struct {
	SHA         string
	CommittedAt time.Time
}

// ActivityInput is the input to calculation and snapshot normalization. The
// enclosing GitData supplies the pinned repository and refs. Window.Until is t.
// A partial source may supply observations and an independently proven last commit.
type ActivityInput struct {
	Window      Window
	Source      SourceState
	Commits     []ActivityCommit
	FirstCommit *ActivityCommit
	LastCommit  *ActivityCommit
	Periods     []int // Complete seven-day periods, oldest to newest, ending at t.
	// HistoryAgeDays is the number of full UTC days since FirstCommit at t.
	// It is nil when the history is empty or incomplete.
	HistoryAgeDays *int
}

// Activity uses the accepted analysis window, never the wall clock at collection.
// FirstCommit and LastCommit describe the history through t, even outside the window.
func (d GitData) Activity(window Window) ActivityInput {
	window = Window{From: window.From.UTC(), Until: window.Until.UTC()}
	input := ActivityInput{
		Window: window, Commits: []ActivityCommit{}, Periods: []int{},
		Source: SourceState{Source: "git", Scope: ActivityHistoryScope, Status: "unavailable", Reason: &Reason{
			Code: "GIT_HISTORY_INTERRUPTED", Message: "История основной ветки не получена",
			CannotConclude: "Нельзя подтвердить активность основной ветки",
		}},
	}
	for _, state := range d.Sources {
		if state.Source == "git" && state.Scope == ActivityHistoryScope {
			input.Source = state
			break
		}
	}
	seen := make(map[string]struct{}, len(d.MainCommits))
	var earliest, latest *ActivityCommit
	valid, invalid := 0, 0
	for _, commit := range d.MainCommits {
		commit.CommittedAt = commit.CommittedAt.UTC()
		at := commit.CommittedAt
		if _, exists := seen[commit.SHA]; exists {
			continue
		}
		seen[commit.SHA] = struct{}{}
		if at.IsZero() || at.After(window.Until) {
			invalid++
			continue
		}
		valid++
		if !at.Before(window.From) && at.Before(window.Until) {
			input.Commits = append(input.Commits, commit)
		}
		if earliest == nil || at.Before(earliest.CommittedAt) || at.Equal(earliest.CommittedAt) && commit.SHA < earliest.SHA {
			earliest = &commit
		}
		if latest == nil || at.After(latest.CommittedAt) || at.Equal(latest.CommittedAt) && commit.SHA < latest.SHA {
			latest = &commit
		}
	}
	if invalid > 0 {
		input.Source.Status = "partial"
		if valid == 0 {
			input.Source.Status = "unavailable"
		}
		const code = "GIT_HISTORY_INVALID"
		input.Source.Sample.Collected = &valid
		input.Source.Sample.Total = nil
		input.Source.Sample.Excluded = &invalid
		reason := code
		input.Source.Sample.ExcludedReason = &reason
		input.Source.Fingerprint = nil
		if input.Source.Reason == nil {
			input.Source.Reason = &Reason{Code: code, Message: "В истории есть недопустимые даты коммитов",
				CannotConclude: "Полнота истории на момент оценки не подтверждена"}
		}
	}
	if invalid == 0 && (input.Source.Status == "complete" || d.MainWalkComplete) {
		input.LastCommit = latest
	}
	if input.Source.Status == "complete" {
		input.FirstCommit = earliest
	}
	if first := input.FirstCommit; first != nil {
		seconds := window.Until.Unix() - first.CommittedAt.Unix()
		if window.Until.Nanosecond() < first.CommittedAt.Nanosecond() {
			seconds--
		}
		age := int(seconds / (24 * 60 * 60))
		input.HistoryAgeDays = &age
		const week = 7 * 24 * time.Hour
		weeks := min(age/7, int(window.Until.Sub(window.From)/week))
		if weeks > 0 {
			input.Periods = make([]int, weeks)
			start := window.Until.Add(-time.Duration(weeks) * week)
			for _, commit := range input.Commits {
				if !commit.CommittedAt.Before(start) {
					input.Periods[int(commit.CommittedAt.Sub(start)/week)]++
				}
			}
		}
	}
	return input
}
