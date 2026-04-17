package recommendation

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/perception"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

const (
	negativeCooldown         = 5 * time.Minute
	ignoreCooldown           = 2 * time.Minute
	recommendationRecordTTL  = 15 * time.Minute
	recommendationStateTTL   = 30 * time.Minute
	maxRecommendationRecords = 256
	maxFingerprintStates     = 128
)

type Service struct {
	mu     sync.Mutex
	now    func() time.Time
	nextID uint64
	items  map[string]record
	state  map[string]fingerprintState
}

type GenerateInput struct {
	Source          string
	Scene           string
	PageTitle       string
	PageURL         string
	AppName         string
	WindowTitle     string
	VisibleText     string
	ScreenSummary   string
	SelectionText   string
	ClipboardText   string
	ClipboardMime   string
	HoverTarget     string
	LastAction      string
	ErrorText       string
	DwellMillis     int
	WindowSwitches  int
	PageSwitches    int
	CopyCount       int
	Signals         perception.SignalSnapshot
	UnfinishedTasks []runengine.TaskRecord
	FinishedTasks   []runengine.TaskRecord
	NotepadItems    []map[string]any
}

type GenerateResult struct {
	CooldownHit bool
	Items       []map[string]any
}

type record struct {
	ID          string
	Fingerprint string
	IntentName  string
	CreatedAt   time.Time
}

type fingerprintState struct {
	CooldownUntil time.Time
	IntentScores  map[string]int
	LastFeedback  map[string]string
	LastTouched   time.Time
}

type candidate struct {
	IntentName string
	Text       string
	Intent     map[string]any
	Priority   int
}

func NewService() *Service {
	return &Service{
		now:   time.Now,
		items: map[string]record{},
		state: map[string]fingerprintState{},
	}
}

func (s *Service) Get(input GenerateInput) GenerateResult {
	fingerprint := recommendationFingerprint(input)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.pruneLocked(now)
	currentState := s.state[fingerprint]
	if currentState.CooldownUntil.After(now) {
		return GenerateResult{
			CooldownHit: true,
			Items:       []map[string]any{},
		}
	}

	candidates := s.rankCandidates(buildCandidates(input), currentState)
	items := make([]map[string]any, 0, len(candidates))
	for _, item := range candidates {
		s.nextID++
		recommendationID := fmt.Sprintf("rec_%03d", s.nextID)
		s.items[recommendationID] = record{
			ID:          recommendationID,
			Fingerprint: fingerprint,
			IntentName:  item.IntentName,
			CreatedAt:   now,
		}
		items = append(items, map[string]any{
			"recommendation_id": recommendationID,
			"text":              item.Text,
			"intent":            item.Intent,
		})
	}
	currentState.LastTouched = now
	s.state[fingerprint] = currentState

	return GenerateResult{
		CooldownHit: false,
		Items:       items,
	}
}

func (s *Service) SubmitFeedback(recommendationID, feedback string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.pruneLocked(now)
	item, ok := s.items[recommendationID]
	if !ok {
		return false
	}

	currentState := s.state[item.Fingerprint]
	if currentState.IntentScores == nil {
		currentState.IntentScores = map[string]int{}
	}
	if currentState.LastFeedback == nil {
		currentState.LastFeedback = map[string]string{}
	}

	switch feedback {
	case "positive":
		currentState.IntentScores[item.IntentName]++
		currentState.CooldownUntil = time.Time{}
	case "negative":
		currentState.IntentScores[item.IntentName]--
		currentState.CooldownUntil = now.Add(negativeCooldown)
	case "ignore":
		currentState.CooldownUntil = now.Add(ignoreCooldown)
	default:
		return false
	}

	currentState.LastFeedback[item.IntentName] = feedback
	currentState.LastTouched = now
	s.state[item.Fingerprint] = currentState
	delete(s.items, recommendationID)
	return true
}

func (s *Service) rankCandidates(candidates []candidate, state fingerprintState) []candidate {
	if len(candidates) == 0 {
		return nil
	}

	filtered := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		filtered = append(filtered, item)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		leftScore := state.IntentScores[filtered[i].IntentName]
		rightScore := state.IntentScores[filtered[j].IntentName]
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority > filtered[j].Priority
		}
		return false
	})

	if len(filtered) > 2 {
		filtered = filtered[:2]
	}
	return filtered
}

func buildCandidates(input GenerateInput) []candidate {
	candidates := make([]candidate, 0, 6)
	selectionText := strings.TrimSpace(input.SelectionText)
	pageTitle := fallbackString(strings.TrimSpace(input.PageTitle), "当前页面")
	perceptionCandidates := make([]candidate, 0, 3)
	signals := input.perceptionSignals()
	for _, opportunity := range perception.IdentifyOpportunities(signals, input.UnfinishedTasks, input.NotepadItems) {
		perceptionCandidates = append(perceptionCandidates, candidate{
			IntentName: opportunity.IntentName,
			Text:       opportunity.Text,
			Intent:     intentPayload(opportunity.IntentName),
			Priority:   opportunity.Priority,
		})
	}

	switch input.Scene {
	case "error":
		candidates = append(candidates, candidate{
			IntentName: "explain",
			Text:       fmt.Sprintf("要不要我先解释一下这个错误和处理方向：%s", truncateText(pageTitle, 18)),
			Intent:     intentPayload("explain"),
			Priority:   1000,
		})
	case "selected_text":
		if selectionText != "" {
			primaryIntent := "explain"
			primaryText := fmt.Sprintf("要不要我先解释这段内容：%s", truncateText(selectionText, 18))
			if utf8.RuneCountInString(selectionText) >= 80 || strings.Contains(selectionText, "\n") {
				primaryIntent = "summarize"
				primaryText = fmt.Sprintf("要不要我先总结这段内容：%s", truncateText(selectionText, 18))
			}
			candidates = append(candidates, candidate{
				IntentName: primaryIntent,
				Text:       primaryText,
				Intent:     intentPayload(primaryIntent),
				Priority:   1000,
			})
			candidates = append(candidates, candidate{
				IntentName: "rewrite",
				Text:       "也可以直接帮你改写成更正式的版本。",
				Intent:     intentPayload("rewrite"),
				Priority:   950,
			})
		}
	default:
		if task, ok := firstUnfinishedTask(input.UnfinishedTasks); ok {
			intentName := taskIntentName(task.Intent)
			candidates = append(candidates, candidate{
				IntentName: intentName,
				Text:       fmt.Sprintf("继续处理当前任务：%s", truncateText(task.Title, 18)),
				Intent:     intentPayload(intentName),
				Priority:   700,
			})
		}
		if item, ok := firstOpenNotepadItem(input.NotepadItems); ok {
			title := stringValue(item, "title")
			intentName := inferNotepadIntent(item)
			candidates = append(candidates, candidate{
				IntentName: intentName,
				Text:       fmt.Sprintf("要不要先处理这个待办：%s", truncateText(title, 18)),
				Intent:     intentPayload(intentName),
				Priority:   650,
			})
		}
	}
	candidates = append(candidates, perceptionCandidates...)

	if len(candidates) == 0 {
		candidates = append(candidates, candidate{
			IntentName: "summarize",
			Text:       fmt.Sprintf("要不要我先整理一下：%s", truncateText(pageTitle, 18)),
			Intent:     intentPayload("summarize"),
			Priority:   500,
		})
	}

	return dedupeCandidates(candidates)
}

func recommendationFingerprint(input GenerateInput) string {
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(input.Source),
		strings.TrimSpace(input.Scene),
		strings.TrimSpace(input.PageTitle),
		strings.TrimSpace(input.AppName),
		strings.TrimSpace(input.SelectionText),
		perception.SignalFingerprint(input.perceptionSignals()),
		taskContextFingerprint(input.UnfinishedTasks),
		notepadContextFingerprint(input.NotepadItems),
	}, "|"))
}

func (input GenerateInput) perceptionSignals() perception.SignalSnapshot {
	if strings.TrimSpace(input.Signals.Source) != "" || strings.TrimSpace(input.Signals.PageTitle) != "" || strings.TrimSpace(input.Signals.ClipboardText) != "" || input.Signals.DwellMillis > 0 || input.Signals.CopyCount > 0 || input.Signals.WindowSwitchCount > 0 || input.Signals.PageSwitchCount > 0 {
		return input.Signals
	}
	return perception.SignalSnapshot{
		Source:            input.Source,
		Scene:             input.Scene,
		PageTitle:         input.PageTitle,
		PageURL:           input.PageURL,
		AppName:           input.AppName,
		WindowTitle:       input.WindowTitle,
		VisibleText:       input.VisibleText,
		ScreenSummary:     input.ScreenSummary,
		SelectionText:     input.SelectionText,
		ClipboardText:     input.ClipboardText,
		ClipboardMimeType: input.ClipboardMime,
		HoverTarget:       input.HoverTarget,
		LastAction:        input.LastAction,
		ErrorText:         input.ErrorText,
		DwellMillis:       input.DwellMillis,
		WindowSwitchCount: input.WindowSwitches,
		PageSwitchCount:   input.PageSwitches,
		CopyCount:         input.CopyCount,
	}
}

func taskContextFingerprint(tasks []runengine.TaskRecord) string {
	parts := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.TaskID == "" {
			continue
		}
		parts = append(parts, strings.ToLower(strings.Join([]string{
			task.TaskID,
			task.Status,
			taskIntentName(task.Intent),
		}, ":")))
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, ",")
}

func notepadContextFingerprint(items []map[string]any) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		itemID := strings.TrimSpace(stringValue(item, "item_id"))
		if itemID == "" {
			continue
		}
		parts = append(parts, strings.ToLower(strings.Join([]string{
			itemID,
			stringValue(item, "bucket"),
			stringValue(item, "status"),
			inferNotepadIntent(item),
		}, ":")))
		if len(parts) == 3 {
			break
		}
	}
	return strings.Join(parts, ",")
}

func (s *Service) pruneLocked(now time.Time) {
	for recommendationID, item := range s.items {
		if now.Sub(item.CreatedAt) > recommendationRecordTTL {
			delete(s.items, recommendationID)
		}
	}
	if len(s.items) > maxRecommendationRecords {
		s.trimRecommendationRecordsLocked()
	}

	for fingerprint, item := range s.state {
		lastTouched := item.LastTouched
		if lastTouched.IsZero() {
			lastTouched = item.CooldownUntil
		}
		if !lastTouched.IsZero() && now.Sub(lastTouched) > recommendationStateTTL {
			delete(s.state, fingerprint)
		}
	}
	if len(s.state) > maxFingerprintStates {
		s.trimFingerprintStatesLocked()
	}
}

func (s *Service) trimRecommendationRecordsLocked() {
	records := make([]record, 0, len(s.items))
	for _, item := range s.items {
		records = append(records, item)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt.Before(records[j].CreatedAt)
	})
	for len(records) > maxRecommendationRecords {
		delete(s.items, records[0].ID)
		records = records[1:]
	}
}

func (s *Service) trimFingerprintStatesLocked() {
	type stateRecord struct {
		fingerprint string
		lastTouched time.Time
	}
	records := make([]stateRecord, 0, len(s.state))
	for fingerprint, item := range s.state {
		lastTouched := item.LastTouched
		if lastTouched.IsZero() {
			lastTouched = item.CooldownUntil
		}
		records = append(records, stateRecord{fingerprint: fingerprint, lastTouched: lastTouched})
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].lastTouched.Before(records[j].lastTouched)
	})
	for len(records) > maxFingerprintStates {
		delete(s.state, records[0].fingerprint)
		records = records[1:]
	}
}

func dedupeCandidates(candidates []candidate) []candidate {
	seen := map[string]struct{}{}
	result := make([]candidate, 0, len(candidates))
	for _, item := range candidates {
		if _, ok := seen[item.IntentName]; ok {
			continue
		}
		seen[item.IntentName] = struct{}{}
		result = append(result, item)
	}
	return result
}

func firstUnfinishedTask(tasks []runengine.TaskRecord) (runengine.TaskRecord, bool) {
	for _, task := range tasks {
		if task.Status == "completed" || task.Status == "cancelled" || task.Status == "failed" || task.Status == "ended_unfinished" {
			continue
		}
		return task, true
	}
	return runengine.TaskRecord{}, false
}

func firstOpenNotepadItem(items []map[string]any) (map[string]any, bool) {
	for _, item := range items {
		status := stringValue(item, "status")
		if status == "completed" || status == "cancelled" {
			continue
		}
		return item, true
	}
	return nil, false
}

func inferNotepadIntent(item map[string]any) string {
	combined := strings.ToLower(stringValue(item, "title") + " " + stringValue(item, "agent_suggestion"))
	switch {
	case strings.Contains(combined, "translate") || strings.Contains(combined, "翻译"):
		return "translate"
	case strings.Contains(combined, "rewrite") || strings.Contains(combined, "改写"):
		return "rewrite"
	case strings.Contains(combined, "explain") || strings.Contains(combined, "解释"):
		return "explain"
	default:
		return "summarize"
	}
}

func taskIntentName(intent map[string]any) string {
	switch name := stringValue(intent, "name"); name {
	case "rewrite", "translate", "explain":
		return name
	default:
		return "summarize"
	}
}

func intentPayload(name string) map[string]any {
	switch name {
	case "rewrite":
		return map[string]any{
			"name": "rewrite",
			"arguments": map[string]any{
				"tone": "professional",
			},
		}
	case "translate":
		return map[string]any{
			"name": "translate",
			"arguments": map[string]any{
				"target_language": "en",
			},
		}
	case "explain":
		return map[string]any{
			"name":      "explain",
			"arguments": map[string]any{},
		}
	default:
		return map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		}
	}
}

func truncateText(value string, maxLength int) string {
	if utf8.RuneCountInString(value) <= maxLength {
		return value
	}

	runes := []rune(value)
	return string(runes[:maxLength]) + "..."
}

func fallbackString(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func stringValue(values map[string]any, key string) string {
	rawValue, ok := values[key]
	if !ok {
		return ""
	}

	value, ok := rawValue.(string)
	if !ok {
		return ""
	}

	return value
}
