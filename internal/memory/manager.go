package memorymanager

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cialloclaw/internal/protocol"
	"cialloclaw/internal/runtime/id"
	logstate "cialloclaw/internal/state/log"
	"cialloclaw/internal/state/memory"
	"cialloclaw/internal/state/task"
)

type Manager struct {
	DataDir    string
	MemoryRepo memory.Repository
	TaskRepo   task.Repository
	LogRepo    logstate.Repository
}

func (m *Manager) ID() string {
	return "state.memory_manager"
}

func (m *Manager) SubscribeTypes() []string {
	return []string{protocol.EventTypeTaskExecutionComplete}
}

func (m *Manager) Handle(ctx context.Context, event protocol.Event) error {
	if event.TaskID == nil {
		return nil
	}
	item, err := m.TaskRepo.GetByID(ctx, *event.TaskID)
	if err != nil || item == nil {
		return err
	}
	summary := item.Summary
	if summary == "" {
		summary = "任务已完成。"
	}
	episode := &memory.EpisodicMemory{
		ID:         id.New("episode"),
		SessionID:  item.SessionID,
		TaskID:     &item.ID,
		Summary:    fmt.Sprintf("%s：%s", item.Title, summary),
		Importance: 0.6,
		Tags:       []string{string(item.Kind), item.Metadata["intent"]},
		CreatedAt:  time.Now(),
	}
	if err := m.MemoryRepo.StoreEpisode(ctx, episode); err != nil {
		return err
	}
	profile, err := m.MemoryRepo.GetProfile(ctx)
	if err != nil {
		return err
	}
	if profile == nil {
		profile = &memory.Profile{ID: id.New("profile"), UpdatedAt: time.Now()}
	}
	action := item.Metadata["intent"]
	if action != "" {
		profile.Facts = uniqueStrings(append(profile.Facts, "最近完成动作："+action))
	}
	if action == "translate" {
		profile.Preferences = uniqueStrings(append(profile.Preferences, "倾向使用翻译辅助"))
	}
	profile.UpdatedAt = time.Now()
	if err := m.MemoryRepo.SaveProfile(ctx, profile); err != nil {
		return err
	}
	return m.exportFiles(ctx)
}

func (m *Manager) exportFiles(ctx context.Context) error {
	episodes, err := m.MemoryRepo.ListEpisodes(ctx)
	if err != nil {
		return err
	}
	profile, err := m.MemoryRepo.GetProfile(ctx)
	if err != nil {
		return err
	}

	sort.Slice(episodes, func(i, j int) bool {
		return episodes[i].CreatedAt.After(episodes[j].CreatedAt)
	})

	var summaryLines []string
	summaryLines = append(summaryLines, "# Session Summary", "")
	for i, episode := range episodes {
		if i == 10 {
			break
		}
		summaryLines = append(summaryLines, "- "+episode.Summary)
	}
	if len(episodes) == 0 {
		summaryLines = append(summaryLines, "- 暂无历史任务。")
	}

	userLines := []string{"# USER", ""}
	if profile != nil {
		userLines = append(userLines, "## Facts")
		if len(profile.Facts) == 0 {
			userLines = append(userLines, "- 暂无")
		}
		for _, fact := range profile.Facts {
			userLines = append(userLines, "- "+fact)
		}
		userLines = append(userLines, "", "## Preferences")
		if len(profile.Preferences) == 0 {
			userLines = append(userLines, "- 暂无")
		}
		for _, pref := range profile.Preferences {
			userLines = append(userLines, "- "+pref)
		}
	}

	summaryPath := filepath.Join(m.DataDir, "memory", "summary.md")
	userPath := filepath.Join(m.DataDir, "memory", "USER.md")
	if err := os.WriteFile(summaryPath, []byte(strings.Join(summaryLines, "\n")), 0o644); err != nil {
		return err
	}
	return os.WriteFile(userPath, []byte(strings.Join(userLines, "\n")), 0o644)
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range values {
		if _, exists := seen[value]; exists || value == "" {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
