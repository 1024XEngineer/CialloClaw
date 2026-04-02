package model

type APIResponse struct {
	OK      bool        `json:"ok"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

type Weather struct {
	State    string `json:"state"`
	Label    string `json:"label"`
	Hint     string `json:"hint"`
	BandText string `json:"bandText"`
	Tone     string `json:"tone"`
	Accent   string `json:"accent"`
	Texture  string `json:"texture"`
}

type SceneAction struct {
	Label       string `json:"label"`
	Key         string `json:"key"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

type SceneSummary struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Subtitle      string      `json:"subtitle"`
	SceneType     string      `json:"sceneType"`
	WeatherState  string      `json:"weatherState"`
	WeatherLabel  string      `json:"weatherLabel"`
	Tone          string      `json:"tone"`
	Accent        string      `json:"accent"`
	PrimaryAction SceneAction `json:"primaryAction"`
}

type SceneDetail struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	Subtitle      string                 `json:"subtitle"`
	SceneType     string                 `json:"sceneType"`
	Weather       Weather                `json:"weather"`
	StoryLead     string                 `json:"storyLead"`
	Summary       string                 `json:"summary"`
	Tone          string                 `json:"tone"`
	Accent        string                 `json:"accent"`
	PrimaryAction SceneAction            `json:"primaryAction"`
	Content       map[string]interface{} `json:"content"`
}

type BootstrapData struct {
	AppName        string          `json:"appName"`
	Version        string          `json:"version"`
	DefaultSceneID string          `json:"defaultSceneId"`
	DefaultWeather Weather         `json:"defaultWeather"`
	Scenes         []SceneSummary  `json:"scenes"`
	Preferences    UserPreferences `json:"preferences"`
	ServerTime     string          `json:"serverTime"`
}

type UserPreferences struct {
	Headline          string   `json:"headline"`
	WorkWindow        string   `json:"workWindow"`
	InspectionOrder   []string `json:"inspectionOrder"`
	PrimaryHabit      string   `json:"primaryHabit"`
	ContextHabit      string   `json:"contextHabit"`
	RiskBias          string   `json:"riskBias"`
	ClarifyPreference string   `json:"clarifyPreference"`
}

type SituationPriority struct {
	Title       string `json:"title"`
	WhyNow      string `json:"whyNow"`
	Prepared    string `json:"prepared"`
	Consequence string `json:"consequence"`
}

type SituationData struct {
	SceneID       string              `json:"sceneId"`
	Weather       Weather             `json:"weather"`
	Summary       string              `json:"summary"`
	TopPriorities []SituationPriority `json:"topPriorities"`
	Prepared      []string            `json:"prepared"`
	HabitBasis    string              `json:"habitBasis"`
	ActionLabel   string              `json:"actionLabel"`
	Pressure      string              `json:"pressure"`
}

type MemoryHabit struct {
	Scene      string `json:"scene"`
	Cue        string `json:"cue"`
	Basis      string `json:"basis"`
	Confidence string `json:"confidence"`
}

type MemoryData struct {
	Headline   string        `json:"headline"`
	Note       string        `json:"note"`
	Habits     []MemoryHabit `json:"habits"`
	UpdatedAt  string        `json:"updatedAt"`
	Confidence string        `json:"confidence"`
}

type DraftSection struct {
	Header  string   `json:"header"`
	Bullets []string `json:"bullets"`
}

type DraftData struct {
	Title    string         `json:"title"`
	Status   string         `json:"status"`
	Sections []DraftSection `json:"sections"`
	Notes    []string       `json:"notes"`
	NextStep string         `json:"nextStep"`
}

type LogEntry struct {
	Time     string `json:"time"`
	Scene    string `json:"scene"`
	Action   string `json:"action"`
	Result   string `json:"result"`
	Rollback string `json:"rollback"`
}

type AuthorizationLevel struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

type RiskPreview struct {
	SceneID             string               `json:"sceneId"`
	SceneTitle          string               `json:"sceneTitle"`
	ActionKey           string               `json:"actionKey"`
	ActionLabel         string               `json:"actionLabel"`
	RiskLevel           string               `json:"riskLevel"`
	Summary             string               `json:"summary"`
	Impacts             []string             `json:"impacts"`
	Rollbacks           []string             `json:"rollbacks"`
	Irreversible        []string             `json:"irreversible"`
	AuthorizationLevels []AuthorizationLevel `json:"authorizationLevels"`
	RecommendedDepth    string               `json:"recommendedDepth"`
}

type ActionOutcome struct {
	SceneID       string     `json:"sceneId"`
	ActionKey     string     `json:"actionKey"`
	Mode          string     `json:"mode"`
	Message       string     `json:"message"`
	Summary       string     `json:"summary,omitempty"`
	Draft         *DraftData `json:"draft,omitempty"`
	RiskActionKey string     `json:"riskActionKey,omitempty"`
}

type AuthorizationResult struct {
	SceneID     string    `json:"sceneId"`
	SceneTitle  string    `json:"sceneTitle"`
	ActionKey   string    `json:"actionKey"`
	ActionLabel string    `json:"actionLabel"`
	Depth       string    `json:"depth"`
	DepthLabel  string    `json:"depthLabel"`
	Applied     bool      `json:"applied"`
	Message     string    `json:"message"`
	Rollback    string    `json:"rollback"`
	Log         *LogEntry `json:"log,omitempty"`
}
