package orchestrator

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/languagepolicy"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

const maxRememberedFreeInputTurns = 3

type freeInputTurn struct {
	UserText       string
	AssistantReply string
	CreatedAt      time.Time
}

// freeInputSessionState keeps lightweight near-field conversation continuity
// before a formal task exists. It is intentionally separate from task snapshots
// and durable memory because detached chat should only help interpret the next
// session-local input instead of becoming formal task or long-term memory data.
type freeInputSessionState struct {
	Turns         []freeInputTurn
	ReplyLanguage string
	UpdatedAt     time.Time
}

type freeInputSessionContext struct {
	Summary       string
	ReplyLanguage string
}

func (s *Service) rememberedSessionRecallDecision(sessionID string, snapshot taskcontext.TaskContextSnapshot) (inputRouteDecision, bool) {
	if !isRememberableFreeInputSnapshot(snapshot) || !isSessionRecallQuestion(snapshot.Text) {
		return inputRouteDecision{}, false
	}

	state, ok := s.sessionRecallState(sessionID)
	reply := buildSessionRecallReply(snapshot, state, ok)
	if strings.TrimSpace(reply) == "" {
		return inputRouteDecision{}, false
	}

	return inputRouteDecision{
		Route: inputRouteSocialChat,
		Reply: reply,
	}, true
}

func (s *Service) applySessionInputContext(params map[string]any, snapshot taskcontext.TaskContextSnapshot) taskcontext.TaskContextSnapshot {
	if !isRememberableFreeInputSnapshot(snapshot) {
		return snapshot
	}

	sessionID := strings.TrimSpace(stringValue(params, "session_id", ""))
	context, ok := s.sessionFreeInputContext(sessionID)
	if !ok {
		return snapshot
	}

	snapshot.SessionContextText = context.Summary
	snapshot.SessionReplyLanguage = context.ReplyLanguage
	return snapshot
}

// applyRememberedSessionFollowUp upgrades terse referential commands like
// "translate this paragraph" into a structured target snapshot before a formal
// task exists. This keeps recent detached turns available both for direct
// execution and for later confirmation flows without leaking them into normal
// continuation after a task already owns the context.
func (s *Service) applyRememberedSessionFollowUp(params map[string]any, snapshot taskcontext.TaskContextSnapshot, suggestion intent.Suggestion, confirmRequired bool) (taskcontext.TaskContextSnapshot, intent.Suggestion) {
	if s == nil {
		return snapshot, suggestion
	}

	sessionID := strings.TrimSpace(stringValue(params, "session_id", ""))
	targetText, followUpIntent, ok := s.rememberedSessionFollowUpTarget(sessionID, snapshot)
	if !ok {
		return snapshot, suggestion
	}

	updatedSnapshot := snapshot
	updatedSnapshot.SelectionText = targetText
	updatedSuggestion := s.intent.Suggest(updatedSnapshot, followUpIntent, confirmRequired)
	updatedSuggestion = s.normalizeSuggestedIntentForAvailability(updatedSnapshot, updatedSuggestion, confirmRequired)
	if confirmRequired {
		updatedSuggestion.RequiresConfirm = true
	}
	return updatedSnapshot, updatedSuggestion
}

func (s *Service) rememberSessionInputTurn(sessionID string, snapshot taskcontext.TaskContextSnapshot, assistantReply string) {
	if s == nil {
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	userText := rememberedFreeInputUserText(snapshot)
	if sessionID == "" || userText == "" {
		return
	}
	if isSessionRecallQuestion(snapshot.Text) {
		return
	}

	s.sessionInputMu.Lock()
	defer s.sessionInputMu.Unlock()

	now := time.Now()
	replyLanguage := preferredSessionReplyLanguage(snapshot)
	turn := freeInputTurn{
		UserText:       truncateText(userText, 120),
		AssistantReply: truncateText(strings.TrimSpace(assistantReply), 120),
		CreatedAt:      now,
	}

	s.sessionInputs[sessionID] = appendRememberedSessionTurn(s.sessionInputs[sessionID], turn, replyLanguage, now)
	s.sessionRecalls[sessionID] = appendRememberedSessionTurn(s.sessionRecalls[sessionID], turn, replyLanguage, now)
}

func (s *Service) clearSessionInputContext(sessionID string) {
	if s == nil {
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}

	s.sessionInputMu.Lock()
	defer s.sessionInputMu.Unlock()
	delete(s.sessionInputs, sessionID)
}

func (s *Service) sessionRecallState(sessionID string) (freeInputSessionState, bool) {
	if s == nil {
		return freeInputSessionState{}, false
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return freeInputSessionState{}, false
	}

	s.sessionInputMu.Lock()
	defer s.sessionInputMu.Unlock()

	state, ok := s.sessionRecalls[sessionID]
	if !ok {
		return freeInputSessionState{}, false
	}
	if freeInputSessionStateExpired(state) {
		delete(s.sessionRecalls, sessionID)
		return freeInputSessionState{}, false
	}
	return cloneFreeInputSessionState(state), true
}

func (s *Service) sessionFreeInputContext(sessionID string) (freeInputSessionContext, bool) {
	if s == nil {
		return freeInputSessionContext{}, false
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return freeInputSessionContext{}, false
	}

	s.sessionInputMu.Lock()
	defer s.sessionInputMu.Unlock()

	state, ok := s.sessionInputs[sessionID]
	if !ok {
		return freeInputSessionContext{}, false
	}
	if freeInputSessionStateExpired(state) {
		delete(s.sessionInputs, sessionID)
		return freeInputSessionContext{}, false
	}

	summary := buildFreeInputSummary(state.Turns)
	if summary == "" {
		return freeInputSessionContext{}, false
	}

	return freeInputSessionContext{
		Summary:       summary,
		ReplyLanguage: firstNonEmptyString(strings.TrimSpace(state.ReplyLanguage), languagepolicy.ReplyLanguageChinese),
	}, true
}

func freeInputSessionStateExpired(state freeInputSessionState) bool {
	if state.UpdatedAt.IsZero() {
		return true
	}
	return time.Since(state.UpdatedAt) > implicitSessionReuseWindow
}

// appendRememberedSessionTurn keeps detached context continuity and short-lived
// user-facing recall history on the same retention policy while allowing task
// creation to clear only the inference-facing store.
func appendRememberedSessionTurn(state freeInputSessionState, turn freeInputTurn, replyLanguage string, now time.Time) freeInputSessionState {
	if freeInputSessionStateExpired(state) {
		state = freeInputSessionState{}
	}

	state.UpdatedAt = now
	state.ReplyLanguage = replyLanguage
	state.Turns = append(state.Turns, turn)
	if len(state.Turns) > maxRememberedFreeInputTurns {
		state.Turns = append([]freeInputTurn(nil), state.Turns[len(state.Turns)-maxRememberedFreeInputTurns:]...)
	}
	return state
}

func buildFreeInputSummary(turns []freeInputTurn) string {
	if len(turns) == 0 {
		return ""
	}

	lines := make([]string, 0, len(turns)*2)
	for _, turn := range turns {
		if text := strings.TrimSpace(turn.UserText); text != "" {
			lines = append(lines, fmt.Sprintf("user: %s", text))
		}
		if text := strings.TrimSpace(turn.AssistantReply); text != "" {
			lines = append(lines, fmt.Sprintf("assistant: %s", text))
		}
	}
	return strings.Join(lines, "\n")
}

func (s *Service) rememberedSessionFollowUpTarget(sessionID string, snapshot taskcontext.TaskContextSnapshot) (string, map[string]any, bool) {
	if !isRememberableFreeInputSnapshot(snapshot) || !canUseRememberedSessionTarget(snapshot) {
		return "", nil, false
	}

	state, ok := s.sessionRecallState(sessionID)
	if !ok {
		return "", nil, false
	}

	targetText := latestRememberedUserText(state.Turns)
	if strings.TrimSpace(targetText) == "" {
		return "", nil, false
	}

	intentValue, ok := rememberedSessionFollowUpIntent(snapshot.Text)
	if !ok {
		return "", nil, false
	}
	return targetText, intentValue, true
}

func canUseRememberedSessionTarget(snapshot taskcontext.TaskContextSnapshot) bool {
	return strings.TrimSpace(snapshot.SelectionText) == "" &&
		strings.TrimSpace(snapshot.ErrorText) == "" &&
		len(snapshot.Files) == 0
}

func latestRememberedUserText(turns []freeInputTurn) string {
	for index := len(turns) - 1; index >= 0; index-- {
		if text := strings.TrimSpace(turns[index].UserText); text != "" {
			return text
		}
	}
	return ""
}

func rememberedFreeInputUserText(snapshot taskcontext.TaskContextSnapshot) string {
	userText := firstNonEmptyString(snapshot.Text, snapshot.SelectionText)
	return firstNonEmptyString(userText, snapshot.ErrorText)
}

func isRememberableFreeInputSnapshot(snapshot taskcontext.TaskContextSnapshot) bool {
	return snapshot.InputType == "text" &&
		strings.TrimSpace(snapshot.Text) != "" &&
		len(snapshot.Files) == 0 &&
		strings.TrimSpace(snapshot.SelectionText) == "" &&
		strings.TrimSpace(snapshot.ErrorText) == ""
}

func preferredSessionReplyLanguage(snapshot taskcontext.TaskContextSnapshot) string {
	if shouldPreferRememberedSessionLanguage(snapshot, languagepolicy.PreferredReplyLanguage(snapshot.Text)) {
		return strings.TrimSpace(snapshot.SessionReplyLanguage)
	}

	preferredInput := rememberedFreeInputUserText(snapshot)
	if preferredInput == "" {
		preferredInput = snapshot.SessionContextText
	}
	return languagepolicy.PreferredReplyLanguage(preferredInput)
}

func shouldPreferRememberedSessionLanguage(snapshot taskcontext.TaskContextSnapshot, currentLanguage string) bool {
	rememberedLanguage := strings.TrimSpace(snapshot.SessionReplyLanguage)
	if rememberedLanguage == "" || rememberedLanguage == currentLanguage {
		return false
	}
	if currentLanguage != languagepolicy.ReplyLanguageEnglish || rememberedLanguage != languagepolicy.ReplyLanguageChinese {
		return false
	}
	if !isRememberableFreeInputSnapshot(snapshot) || strings.TrimSpace(snapshot.SessionContextText) == "" {
		return false
	}
	return utf8.RuneCountInString(strings.TrimSpace(snapshot.Text)) <= 32
}

func rememberedSessionFollowUpIntent(text string) (map[string]any, bool) {
	normalized := strings.TrimSpace(text)
	if normalized == "" || !isRememberedSessionReferenceText(normalized) {
		return nil, false
	}
	if intentValue, ok := heuristicTranslateIntentFromCorrection(normalized); ok {
		return intentValue, true
	}
	return nil, false
}

func isRememberedSessionReferenceText(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	for _, marker := range []string{
		"this",
		"that",
		"it",
		"above",
		"previous",
		"last paragraph",
		"这段",
		"这段话",
		"上面",
		"上一段",
		"前面那段",
		"刚才那段",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return utf8.RuneCountInString(normalized) <= 16
}

func cloneFreeInputSessionState(state freeInputSessionState) freeInputSessionState {
	cloned := state
	if len(state.Turns) > 0 {
		cloned.Turns = append([]freeInputTurn(nil), state.Turns...)
	}
	return cloned
}

func buildSessionRecallReply(snapshot taskcontext.TaskContextSnapshot, state freeInputSessionState, hasState bool) string {
	replyLanguage := preferredSessionReplyLanguage(snapshot)
	if !hasState || len(state.Turns) == 0 {
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			return "I do not have any earlier detached chat cached in this session yet."
		}
		return "我这边暂时还没有可用的上一轮对话缓存。"
	}

	lastTurn := state.Turns[len(state.Turns)-1]
	trimmedInput := strings.TrimSpace(snapshot.Text)
	switch {
	case asksForAssistantRecall(trimmedInput):
		if strings.TrimSpace(lastTurn.AssistantReply) == "" {
			if replyLanguage == languagepolicy.ReplyLanguageEnglish {
				return fmt.Sprintf("I remember your last message was: %q.", lastTurn.UserText)
			}
			return fmt.Sprintf("我记得你上一句是：%q。", lastTurn.UserText)
		}
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			return fmt.Sprintf("My last reply was: %q.", lastTurn.AssistantReply)
		}
		return fmt.Sprintf("我上一句回复是：%q。", lastTurn.AssistantReply)
	case asksForRecentUserHistory(trimmedInput):
		userTurns := recentRememberedUserTurns(state.Turns, 3)
		if len(userTurns) == 0 {
			if replyLanguage == languagepolicy.ReplyLanguageEnglish {
				return "I do not have any earlier user message cached in this session yet."
			}
			return "我这边暂时还没有可用的上一轮用户输入缓存。"
		}
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			return "I remember your recent messages: " + strings.Join(userTurns, " | ")
		}
		return "我记得你最近说过：" + strings.Join(userTurns, "；")
	default:
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			if strings.TrimSpace(lastTurn.AssistantReply) == "" {
				return fmt.Sprintf("Your last message was: %q.", lastTurn.UserText)
			}
			return fmt.Sprintf("Your last message was: %q. My last reply was: %q.", lastTurn.UserText, lastTurn.AssistantReply)
		}
		if strings.TrimSpace(lastTurn.AssistantReply) == "" {
			return fmt.Sprintf("你上一句是：%q。", lastTurn.UserText)
		}
		return fmt.Sprintf("你上一句是：%q。我上一句回复是：%q。", lastTurn.UserText, lastTurn.AssistantReply)
	}
}

func recentRememberedUserTurns(turns []freeInputTurn, limit int) []string {
	if len(turns) == 0 || limit <= 0 {
		return nil
	}

	start := len(turns) - limit
	if start < 0 {
		start = 0
	}

	items := make([]string, 0, len(turns)-start)
	for _, turn := range turns[start:] {
		if text := strings.TrimSpace(turn.UserText); text != "" {
			items = append(items, fmt.Sprintf("%q", text))
		}
	}
	return items
}

func isSessionRecallQuestion(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	return asksForPreviousUserMessage(normalized) || asksForAssistantRecall(normalized) || asksForRecentUserHistory(normalized)
}

func asksForPreviousUserMessage(text string) bool {
	for _, marker := range []string{
		"上句话",
		"上一句",
		"上一条",
		"我刚才说了什么",
		"我上一句说了什么",
		"what was my last message",
		"what did i just say",
		"my previous message",
		"my last sentence",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func asksForAssistantRecall(text string) bool {
	for _, marker := range []string{
		"你刚才说了什么",
		"你上一句说了什么",
		"你刚刚怎么回复的",
		"你怎么回答的",
		"what did you just say",
		"what was your last reply",
		"your previous reply",
		"your last message",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func asksForRecentUserHistory(text string) bool {
	for _, marker := range []string{
		"此前都说了什么",
		"之前都说了什么",
		"我都说了什么",
		"我说过什么",
		"聊天记录",
		"上一轮对话",
		"recent messages",
		"conversation history",
		"what have i said",
		"what did i say before",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}
