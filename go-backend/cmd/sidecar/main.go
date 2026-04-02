package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

type HomeStatus struct {
	Status   string `json:"status"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Mood     string `json:"mood"`
}

type NudgeAction struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Intent string `json:"intent"`
	Tone   string `json:"tone"`
}

type NudgeItem struct {
	ID      string        `json:"id"`
	Level   string        `json:"level"`
	Title   string        `json:"title"`
	Summary string        `json:"summary"`
	Scene   string        `json:"scene"`
	Actions []NudgeAction `json:"actions"`
}

type ScenarioItem struct {
	ID              string `json:"id"`
	Category        string `json:"category"`
	Title           string `json:"title"`
	Summary         string `json:"summary"`
	Detail          string `json:"detail"`
	Risk            string `json:"risk"`
	SuggestedAction string `json:"suggestedAction"`
}

type SettingsItem struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Value       any      `json:"value"`
	Options     []string `json:"options,omitempty"`
}

type SettingsSection struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Items       []SettingsItem `json:"items"`
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

func main() {
	port := os.Getenv("CIALLOCLAW_PORT")
	if port == "" {
		port = "47831"
	}

	http.HandleFunc("/api/home", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, HomeStatus{
			Status:   "helping",
			Title:    "桌面常驻陪伴中",
			Subtitle: "低打扰主动协助",
			Mood:     "伴",
		})
	})

	http.HandleFunc("/api/nudges", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []NudgeItem{
			{
				ID:      "nudge-summary",
				Level:   "L1",
				Title:   "内容理解机会",
				Summary: "检测到一段较长的技术材料，已准备好摘要、重点和下一步建议。",
				Scene:   "内容理解与总结",
				Actions: []NudgeAction{{ID: "view", Label: "查看", Intent: "view", Tone: "primary"}, {ID: "later", Label: "稍后", Intent: "later", Tone: "secondary"}, {ID: "ignore", Label: "忽略", Intent: "ignore", Tone: "ghost"}},
			},
			{
				ID:      "nudge-todo",
				Level:   "L2",
				Title:   "待办巡检提醒",
				Summary: "今天还有 1 个高优先级事项未处理，建议先查看确认后再执行。",
				Scene:   "待办巡检与提醒",
				Actions: []NudgeAction{{ID: "view", Label: "查看", Intent: "view", Tone: "primary"}},
			},
		})
	})

	http.HandleFunc("/api/scenarios", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []ScenarioItem{
			{ID: "summary", Category: "内容理解", Title: "内容理解与总结", Summary: "从长文、网页和文档里提炼重点，生成简短结论与行动建议。", Detail: "Go sidecar 返回的 mock 结果会像真实 Agent 一样给出摘要、结构化要点与建议回复草稿。", Risk: "green", SuggestedAction: "生成摘要"},
			{ID: "todo", Category: "待办管家", Title: "待办巡检与提醒", Summary: "围绕时间线索与状态变化做低打扰提醒，不强打断。", Detail: "原型以静态待办样本模拟巡检结果，包括临期事项、优先级排序与建议推进步骤。", Risk: "yellow", SuggestedAction: "生成提醒草稿"},
			{ID: "exception", Category: "问题排查", Title: "异常排查与解释", Summary: "对错误界面、终端报错和异常现象给出解释与下一步排查建议。", Detail: "当前展示 mock 排查路径，包括原因分层、影响范围和下一步动作。", Risk: "yellow", SuggestedAction: "解释异常"},
			{ID: "memory", Category: "镜子记忆", Title: "镜子记忆 / 长期协作感", Summary: "展示长期协作感：记住用户偏好、近期重点与习惯节奏。", Detail: "原型里的记忆信息全部由 Go sidecar mock 提供，后续可替换为真实 memory / policy 服务。", Risk: "green", SuggestedAction: "查看记忆摘要"},
			{ID: "safety", Category: "安全卫士", Title: "安全确认 / 风险等级展示", Summary: "高风险动作必须确认，展示红黄绿灯式风险说明。", Detail: "当前演示的是黄灯确认流：先解释对象与影响，再允许用户确认执行 mock 动作。", Risk: "red", SuggestedAction: "确认执行"},
		})
	})

	http.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []SettingsSection{
			{
				ID:          "appearance",
				Title:       "外观与桌面形态",
				Description: "保持轻巧、像素风、桌面原生感。",
				Items: []SettingsItem{
					{ID: "theme", Label: "主题", Description: "当前原型默认浅色奶油桌面主题。", Type: "choice", Value: "奶油像素", Options: []string{"奶油像素", "夜间柔和"}},
					{ID: "orb-stick", Label: "悬浮球边缘吸附", Description: "拖拽后贴边停靠，减少桌面打扰。", Type: "toggle", Value: true},
				},
			},
			{
				ID:          "behavior",
				Title:       "主动协助策略",
				Description: "围绕低打扰、先提示再确认的原则做行为控制。",
				Items: []SettingsItem{
					{ID: "nudge", Label: "轻提示开关", Description: "允许围绕桌面任务机会展示轻提示卡片。", Type: "toggle", Value: true},
					{ID: "memory", Label: "镜子记忆说明", Description: "当前仅为 mock 展示，不写入真实个人数据。", Type: "info", Value: "mock-only"},
				},
			},
			{
				ID:          "security",
				Title:       "安全与权限",
				Description: "展示未来 policy / workflow / memory 接入边界。",
				Items: []SettingsItem{
					{ID: "mock-mode", Label: "Mock 模式", Description: "所有执行结果来自 Go sidecar mock，不触达真实系统。", Type: "toggle", Value: true},
					{ID: "risk", Label: "高风险动作", Description: "始终走确认流，并展示风险等级。", Type: "choice", Value: "始终确认", Options: []string{"始终确认"}},
				},
			},
		})
	})

	http.HandleFunc("/api/actions/confirm", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"ok":      true,
			"message": "已完成 mock 确认动作，下一步可替换为真实 workflow 执行。",
		})
	})

	log.Printf("CialloClaw mock sidecar listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
