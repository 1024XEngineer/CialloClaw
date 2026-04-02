package store

import (
	"fmt"
	"sync"
	"time"

	"cialloclaw.local/backend/internal/data"
	"cialloclaw.local/backend/internal/model"
)

type Store struct {
	mu             sync.RWMutex
	data           *data.MockData
	currentSceneID string
}

func New(mock *data.MockData) *Store {
	return &Store{
		data:           mock,
		currentSceneID: mock.Bootstrap.DefaultSceneID,
	}
}

func (s *Store) Bootstrap() model.BootstrapData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Bootstrap
}

func (s *Store) DefaultSceneID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.currentSceneID != "" {
		return s.currentSceneID
	}
	return s.data.Bootstrap.DefaultSceneID
}

func (s *Store) Scenes() []model.SceneSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.SceneSummary, len(s.data.Bootstrap.Scenes))
	copy(out, s.data.Bootstrap.Scenes)
	return out
}

func (s *Store) SceneDetail(id string) (model.SceneDetail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = s.data.Bootstrap.DefaultSceneID
	}
	s.currentSceneID = id
	item, ok := s.data.Scenes[id]
	return item, ok
}

func (s *Store) Weather(sceneID string) (model.Weather, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sceneID == "" {
		sceneID = s.currentSceneID
	}
	if sceneID == "" {
		sceneID = s.data.Bootstrap.DefaultSceneID
	}
	s.currentSceneID = sceneID
	item, ok := s.data.Scenes[sceneID]
	return item.Weather, ok
}

func (s *Store) Situation(sceneID string) (model.SituationData, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sceneID == "" {
		sceneID = s.currentSceneID
	}
	if sceneID == "" {
		sceneID = s.data.Bootstrap.DefaultSceneID
	}
	s.currentSceneID = sceneID
	item, ok := s.data.Situations[sceneID]
	return item, ok
}

func (s *Store) Memory() model.MemoryData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.Memory
}

func (s *Store) Logs() []model.LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.LogEntry, len(s.data.Logs))
	copy(out, s.data.Logs)
	return out
}

func (s *Store) Propose(sceneID, actionKey string) (model.ActionOutcome, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sceneID == "" {
		sceneID = s.currentSceneID
	}
	if sceneID == "" {
		sceneID = s.data.Bootstrap.DefaultSceneID
	}
	if actionKey == "" {
		if scene, ok := s.data.Scenes[sceneID]; ok {
			actionKey = scene.PrimaryAction.Key
		}
	}
	s.currentSceneID = sceneID
	outcome, ok := s.data.ActionOutcomes[actionKey]
	if !ok {
		if scene, okScene := s.data.Scenes[sceneID]; okScene {
			return model.ActionOutcome{
				SceneID:   sceneID,
				ActionKey: actionKey,
				Mode:      "inline-note",
				Message:   fmt.Sprintf("%s 的建议已就位。", scene.Title),
				Summary:   scene.Summary,
			}, true
		}
		return model.ActionOutcome{}, false
	}
	outcome.SceneID = sceneID
	outcome.ActionKey = actionKey
	return outcome, true
}

func (s *Store) RiskPreview(sceneID, actionKey string) (model.RiskPreview, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sceneID == "" {
		sceneID = s.currentSceneID
	}
	if sceneID == "" {
		sceneID = s.data.Bootstrap.DefaultSceneID
	}
	if actionKey == "" {
		if scene, ok := s.data.Scenes[sceneID]; ok {
			actionKey = scene.PrimaryAction.Key
		}
	}
	s.currentSceneID = sceneID
	preview, ok := s.data.RiskPreviews[actionKey]
	if !ok {
		return model.RiskPreview{}, false
	}
	preview.SceneID = sceneID
	if scene, okScene := s.data.Scenes[sceneID]; okScene {
		preview.SceneTitle = scene.Title
	}
	preview.ActionKey = actionKey
	return preview, true
}

func (s *Store) Authorize(sceneID, actionKey, depth string) (model.AuthorizationResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sceneID == "" {
		sceneID = s.currentSceneID
	}
	if sceneID == "" {
		sceneID = s.data.Bootstrap.DefaultSceneID
	}
	if actionKey == "" {
		if scene, ok := s.data.Scenes[sceneID]; ok {
			actionKey = scene.PrimaryAction.Key
		}
	}
	s.currentSceneID = sceneID
	preview, ok := s.data.RiskPreviews[actionKey]
	if !ok {
		return model.AuthorizationResult{}, false
	}
	if depth == "" {
		depth = preview.RecommendedDepth
	}
	depthText := labelForDepth(depth)
	if depthText == "" {
		depthText = "仅预览"
		depth = "preview"
	}

	result := model.AuthorizationResult{
		SceneID:     sceneID,
		SceneTitle:  preview.SceneTitle,
		ActionKey:   actionKey,
		ActionLabel: preview.ActionLabel,
		Depth:       depth,
		DepthLabel:  depthText,
		Applied:     depth == "execute_once",
	}
	var message string
	var rollback string
	var shouldLog bool
	var resultText string
	switch depth {
	case "preview":
		message = "已仅做预览，不会写入任何变化。"
		rollback = "当前没有写入，因此不需要回滚。"
		resultText = "仅预览"
	case "draft":
		message = "已生成草稿，尚未对外执行。"
		rollback = "草稿可直接丢弃或继续编辑。"
		resultText = "草稿已生成"
		shouldLog = true
	case "execute_once":
		message = "已按授权深度执行一次，回滚点已记录。"
		rollback = "可回退到上一个状态。"
		resultText = "已执行一次"
		shouldLog = true
	default:
		message = "已切到默认预览，不会写入任何变化。"
		rollback = "当前没有写入，因此不需要回滚。"
		resultText = "预览模式"
	}
	result.Message = message
	result.Rollback = rollback

	if shouldLog {
		entry := model.LogEntry{
			Time:     time.Now().Format("15:04"),
			Scene:    preview.SceneTitle,
			Action:   preview.ActionLabel + " · " + depthText,
			Result:   resultText,
			Rollback: rollback,
		}
		s.data.Logs = append(s.data.Logs, entry)
		result.Log = &entry
	}

	return result, true
}

func labelForDepth(depth string) string {
	switch depth {
	case "preview":
		return "仅预览"
	case "draft":
		return "仅生成草稿"
	case "execute_once":
		return "允许执行一次"
	default:
		return ""
	}
}
