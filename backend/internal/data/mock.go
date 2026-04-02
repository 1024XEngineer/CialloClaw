package data

import (
	"time"

	"cialloclaw.local/backend/internal/model"
)

type MockData struct {
	Bootstrap      model.BootstrapData
	Scenes         map[string]model.SceneDetail
	Situations     map[string]model.SituationData
	Memory         model.MemoryData
	Logs           []model.LogEntry
	RiskPreviews   map[string]model.RiskPreview
	ActionOutcomes map[string]model.ActionOutcome
}

func Build(now time.Time) *MockData {
	scenes := buildScenes()
	situations := buildSituations()
	memory := buildMemory(now)
	riskPreviews := buildRiskPreviews()
	actionOutcomes := buildActionOutcomes()
	logs := buildLogs()
	bootstrap := buildBootstrap(now, scenes)

	return &MockData{
		Bootstrap:      bootstrap,
		Scenes:         scenes,
		Situations:     situations,
		Memory:         memory,
		Logs:           logs,
		RiskPreviews:   riskPreviews,
		ActionOutcomes: actionOutcomes,
	}
}

func buildBootstrap(now time.Time, scenes map[string]model.SceneDetail) model.BootstrapData {
	ordered := []string{"web-reading", "contract-review", "debug-triage", "daily-report", "resource-anomaly"}
	summaries := make([]model.SceneSummary, 0, len(ordered))
	for _, id := range ordered {
		item := scenes[id]
		summaries = append(summaries, model.SceneSummary{
			ID:            item.ID,
			Title:         item.Title,
			Subtitle:      item.Subtitle,
			SceneType:     item.SceneType,
			WeatherState:  item.Weather.State,
			WeatherLabel:  item.Weather.Label,
			Tone:          item.Tone,
			Accent:        item.Accent,
			PrimaryAction: item.PrimaryAction,
		})
	}

	return model.BootstrapData{
		AppName:        "CialloClaw Prototype 01 - 桌面天气机",
		Version:        "01-mock",
		DefaultSceneID: "web-reading",
		DefaultWeather: model.Weather{
			State:    "cloudy",
			Label:    "多云",
			Hint:     "正在连接本地天气机，默认进入多云状态。",
			BandText: "默认进入多云状态",
			Tone:     "轻压但稳定",
			Accent:   "#9CB7D8",
			Texture:  "cloud",
		},
		Scenes: summaries,
		Preferences: model.UserPreferences{
			Headline:          "这是按你的习惯判断的",
			WorkWindow:        "16:30 - 19:00",
			InspectionOrder:   []string{"结论", "版本差异", "路径", "上下文"},
			PrimaryHabit:      "先看结论，再看证据",
			ContextHabit:      "处理合同先看版本差异，遇到报错先看路径",
			RiskBias:          "先做雷暴预演，再决定是否执行",
			ClarifyPreference: "上下文不完整时先澄清，不抢着下结论",
		},
		ServerTime: now.Format("2006-01-02 15:04:05"),
	}
}

func buildScenes() map[string]model.SceneDetail {
	return map[string]model.SceneDetail{
		"web-reading": {
			ID:            "web-reading",
			Title:         "网页阅读",
			Subtitle:      "把页面里的重点先提纯",
			SceneType:     "reading",
			Weather:       weather("sunny", "晴", "你在这个页面停了 6 分钟，我可以先提炼重点。", "你在这个页面停了 6 分钟，我可以先提炼重点", "清朗", "#76D8FF", "clear"),
			StoryLead:     "你在这个页面停了 6 分钟，我可以先提炼重点。",
			Summary:       "这页内容适合先提炼结论，再回头看证据。",
			Tone:          "清朗",
			Accent:        "#76D8FF",
			PrimaryAction: action("提炼重点", "extract_summary", "proceed", "把网页内容压缩成阅读纪要"),
			Content: map[string]interface{}{
				"articleTitle":    "《桌面天气机原型观察》",
				"articleSubtitle": "先看结论，再回头看证据",
				"elapsed":         "6 分钟",
				"focusPoints": []interface{}{
					"结论段已经给出明确建议",
					"第二段有一处数据口径需要复核",
					"末尾的参考链接可以先收起",
				},
				"summaryLines": []interface{}{
					"先看结论，再回头看证据。",
					"这页已经出现一个可复述的主旨。",
					"不需要逐行等待，适合先抽取重点。",
				},
				"tabs": []interface{}{
					map[string]interface{}{"title": "当前页面", "state": "已停留 6 分钟"},
					map[string]interface{}{"title": "参考页", "state": "可后续核对"},
					map[string]interface{}{"title": "笔记", "state": "已自动准备"},
				},
				"assistantHint": "先提炼重点，不用一次读完",
				"prepared": []interface{}{
					"系统已把标题、结论、证据三段分开",
					"已准备阅读纪要骨架",
				},
			},
		},
		"contract-review": {
			ID:            "contract-review",
			Title:         "合同审阅",
			Subtitle:      "版本差异和风险条款正在冒雾",
			SceneType:     "contract",
			Weather:       weather("fog", "雾", "这份合同明天要提交，但还有 2 处未确认。", "这份合同明天要提交，但还有 2 处未确认", "朦胧但可控", "#C8D0DF", "mist"),
			StoryLead:     "这份合同明天要提交，但还有 2 处未确认。",
			Summary:       "版本差异集中在付款条款和交付边界。",
			Tone:          "朦胧但可控",
			Accent:        "#C8D0DF",
			PrimaryAction: action("生成确认稿", "generate_confirmation_draft", "risk-preview", "先做雷暴预演，再决定是否外发"),
			Content: map[string]interface{}{
				"versionLeft":  "V1.8",
				"versionRight": "V1.9",
				"deadline":     "明天 09:00 前",
				"diffLines": []interface{}{
					map[string]interface{}{"left": "付款周期：30 天", "right": "付款周期：15 天", "kind": "changed", "note": "对外付款节奏变短"},
					map[string]interface{}{"left": "交付范围：按清单", "right": "交付范围：按附件 A", "kind": "changed", "note": "交付边界更具体"},
					map[string]interface{}{"left": "保密条款：一般", "right": "保密条款：加强", "kind": "changed", "note": "敏感信息控制变严"},
				},
				"riskPoints": []interface{}{
					"对外发送前必须确认付款周期",
					"交付范围变化可能影响验收口径",
					"你通常先看版本差异，再看条款边界",
				},
				"habitChecks": []interface{}{
					"你处理合同时通常先看版本差异",
					"你会先确认哪些条款被改了",
					"然后再决定是否发出确认稿",
				},
				"prepared": []interface{}{
					"已将差异按条款类型分组",
					"已把你常看的付款/边界条款置顶",
					"已准备确认稿骨架",
				},
			},
		},
		"debug-triage": {
			ID:            "debug-triage",
			Title:         "报错排查",
			Subtitle:      "路径像被雷劈了一下",
			SceneType:     "debug",
			Weather:       weather("storm", "雷暴", "这条报错更像路径问题，不像权限问题。", "这条报错更像路径问题，不像权限问题", "高压待解", "#8E6BFF", "storm"),
			StoryLead:     "这条报错更像路径问题，不像权限问题。",
			Summary:       "命中路径分支异常，环境变量可以先不动。",
			Tone:          "高压待解",
			Accent:        "#8E6BFF",
			PrimaryAction: action("自动修复建议", "auto_fix", "risk-preview", "先把修复动作放进雷暴预演"),
			Content: map[string]interface{}{
				"errorTitle":   "ModuleNotFoundError: cannot resolve data/record.json",
				"humanExplain": "不是权限拦了，而是拼路径的基准点错了。",
				"traceLines": []interface{}{
					"加载配置 -> resolvePath() -> join(baseDir, data/record.json)",
					"baseDir 指向了可执行文件目录，而不是项目目录",
					"相对路径在这里被压扁",
				},
				"probableCauses": []interface{}{
					"工作目录展开后少了一层",
					"相对路径基准点错位",
					"配置缓存仍在旧路径上",
				},
				"nextSteps": []interface{}{
					"先固定基准目录",
					"再只读验证 record.json",
					"最后再考虑写回",
				},
				"habitChecks": []interface{}{
					"你遇到类似报错时优先检查路径",
					"你通常先验证基准目录",
					"你不喜欢先动权限",
				},
				"prepared": []interface{}{
					"已把路径链路拆成 3 段",
					"已准备只读修复建议",
					"已将回滚点标在当前工作目录",
				},
			},
		},
		"daily-report": {
			ID:            "daily-report",
			Title:         "日报整理",
			Subtitle:      "压力云层正在往桌面边缘压",
			SceneType:     "daily",
			Weather:       weather("shower", "阵雨", "按你平时习惯，这个时间点你通常会开始整理日报。", "按你平时习惯，这个时间点你通常会开始整理日报", "轻压待收尾", "#76D4D0", "shower"),
			StoryLead:     "按你平时习惯，这个时间点你通常会开始整理日报。",
			Summary:       "今天的内容足够拼成一版可提交草稿。",
			Tone:          "轻压待收尾",
			Accent:        "#76D4D0",
			PrimaryAction: action("推进一步", "advance_draft", "proceed", "先把日报草稿起好"),
			Content: map[string]interface{}{
				"completedItems": []interface{}{
					"完成了页面总结的第一版结构",
					"梳理了 2 个可复用的文案段落",
					"把本周风险点分类到待跟进项",
				},
				"followUpItems": []interface{}{
					"补上会议纪要里的两个确认点",
					"把晚些时候的外发版本再扫一次",
					"回看今天修改过的路径问题",
				},
				"habitChecks": []interface{}{
					"按你的习惯，这个时间点你通常会开始写日报",
					"你常会先把完成事项列出来，再补待跟进",
					"你喜欢先搭骨架，再补细节",
				},
				"prepared": []interface{}{
					"已预填日报结构",
					"已把今天的完成项串成三段",
					"已保留你常用的收尾语句",
				},
				"pressureLine": "阵雨带已经压到桌面边缘",
				"draft":        dailyDraft(),
			},
		},
		"resource-anomaly": {
			ID:            "resource-anomaly",
			Title:         "系统资源异常",
			Subtitle:      "看起来是多云，实际上某个进程在升温",
			SceneType:     "resource",
			Weather:       weather("cloudy", "多云", "CPU 在轻微抬升，但还没到失控。", "CPU 在轻微抬升，但还没到失控", "轻压可监测", "#9CB7D8", "cloud"),
			StoryLead:     "CPU 在轻微抬升，但还没到失控。",
			Summary:       "系统还没有失控，但有一个热点进程值得先看。",
			Tone:          "轻压可监测",
			Accent:        "#9CB7D8",
			PrimaryAction: action("聚焦异常源", "inspect_metrics", "proceed", "先把异常缩到 3 个候选项"),
			Content: map[string]interface{}{
				"assistantHint": "系统没崩，只是某个进程在往上拱。",
				"systemNote":    "现在更像轻压力，但还没到强制干预。",
				"metrics": []interface{}{
					map[string]interface{}{"name": "CPU", "value": 72, "unit": "%", "trend": "+18", "label": "近 6 分钟"},
					map[string]interface{}{"name": "内存", "value": 54, "unit": "%", "trend": "+6", "label": "较稳定"},
					map[string]interface{}{"name": "磁盘", "value": 31, "unit": "%", "trend": "+2", "label": "可回滚"},
				},
				"hotspots": []interface{}{
					"渲染进程的 CPU 占用开始上探",
					"后台同步任务刚跑完一轮",
					"本地缓存刷新比平时慢半拍",
				},
				"timeline": []interface{}{
					"09:02 负载开始缓慢爬升",
					"09:05 命中一次缓存刷新",
					"09:06 热点进程开始抬头",
				},
				"prepared": []interface{}{
					"已把异常曲线压到 3 个候选",
					"已预留回滚点",
					"可先做只读观察",
				},
			},
		},
	}
}

func buildSituations() map[string]model.SituationData {
	return map[string]model.SituationData{
		"web-reading": {
			SceneID: "web-reading",
			Weather: weather("sunny", "晴", "你在这个页面停了 6 分钟，我可以先提炼重点。", "你在这个页面停了 6 分钟，我可以先提炼重点", "清朗", "#76D8FF", "clear"),
			Summary: "这页内容适合先提炼结论，再回头看证据。",
			TopPriorities: []model.SituationPriority{
				{Title: "提炼本文重点", WhyNow: "此刻结论最清晰", Prepared: "我已经把结论段和证据段拆开", Consequence: "如果现在不提炼，晚些时候会回到逐句重读"},
				{Title: "标出可复查来源", WhyNow: "这页有一处口径需要核对", Prepared: "已把参考链接挂到旁边", Consequence: "后续复查会变慢"},
				{Title: "生成阅读纪要", WhyNow: "页面停留时间已经足够", Prepared: "纪要骨架已经放好", Consequence: "错过这波上下文，纪要会变粗"},
			},
			Prepared:    []string{"系统已把标题、结论、证据三段分开", "已准备阅读纪要骨架", "已预留一个可继续看证据的位置"},
			HabitBasis:  "你读文章时通常先扫结论，再回头看证据。",
			ActionLabel: "提炼重点",
			Pressure:    "晴",
		},
		"contract-review": {
			SceneID: "contract-review",
			Weather: weather("fog", "雾", "这份合同明天要提交，但还有 2 处未确认。", "这份合同明天要提交，但还有 2 处未确认", "朦胧但可控", "#C8D0DF", "mist"),
			Summary: "版本差异集中在付款条款和交付边界。",
			TopPriorities: []model.SituationPriority{
				{Title: "先看版本差异", WhyNow: "改动边界最需要先确认", Prepared: "已把版本差异分组", Consequence: "拖到最后会搞不清改了什么"},
				{Title: "核对付款条款", WhyNow: "对外发送前最容易出问题", Prepared: "已把付款周期标红", Consequence: "错过会直接影响回款口径"},
				{Title: "整理确认稿", WhyNow: "明天就要提交", Prepared: "确认稿骨架已经在桌面上", Consequence: "不先起草，下午会开始手忙脚乱"},
			},
			Prepared:    []string{"已将差异按条款类型分组", "已把你常看的付款/边界条款置顶", "已准备确认稿骨架"},
			HabitBasis:  "你处理合同时通常先看版本差异，再看条款边界。",
			ActionLabel: "生成确认稿",
			Pressure:    "雾",
		},
		"debug-triage": {
			SceneID: "debug-triage",
			Weather: weather("storm", "雷暴", "这条报错更像路径问题，不像权限问题。", "这条报错更像路径问题，不像权限问题", "高压待解", "#8E6BFF", "storm"),
			Summary: "命中路径分支异常，环境变量可以先不动。",
			TopPriorities: []model.SituationPriority{
				{Title: "验证基准目录", WhyNow: "路径判断的起点错了", Prepared: "已把当前工作目录确认过一遍", Consequence: "继续往下跑只会放大错误"},
				{Title: "只读验证文件", WhyNow: "先确认数据是否真的丢失", Prepared: "已准备只读验证命令", Consequence: "如果先写入，回滚会更难"},
				{Title: "再考虑修复", WhyNow: "路径链路已经足够清晰", Prepared: "回滚点已经标好", Consequence: "不先澄清，修复可能会写偏"},
			},
			Prepared:    []string{"已把路径链路拆成 3 段", "已准备只读修复建议", "已将回滚点标在当前工作目录"},
			HabitBasis:  "你遇到类似报错时优先检查路径。",
			ActionLabel: "自动修复建议",
			Pressure:    "雷暴",
		},
		"daily-report": {
			SceneID: "daily-report",
			Weather: weather("shower", "阵雨", "按你平时习惯，这个时间点你通常会开始整理日报。", "按你平时习惯，这个时间点你通常会开始整理日报", "轻压待收尾", "#76D4D0", "shower"),
			Summary: "今天的内容足够拼成一版可提交草稿。",
			TopPriorities: []model.SituationPriority{
				{Title: "补齐完成事项", WhyNow: "今天的记忆还完整", Prepared: "我已经把 3 个完成动作串起来了", Consequence: "拖到明天会缺上下文"},
				{Title: "整理待跟进项", WhyNow: "两处确认点已经临期", Prepared: "已把待跟进项标红", Consequence: "漏掉后要花更多时间回忆"},
				{Title: "先起日报草稿", WhyNow: "此刻上下文还在桌面上", Prepared: "草稿骨架已经铺好", Consequence: "如果现在不写，后面只能凭记忆补"},
			},
			Prepared:    []string{"已预填日报结构", "已把今天的完成项串成三段", "已保留你常用的收尾语句"},
			HabitBasis:  "这个时间点你通常会先写日报，再补证据。",
			ActionLabel: "推进一步",
			Pressure:    "阵雨",
		},
		"resource-anomaly": {
			SceneID: "resource-anomaly",
			Weather: weather("cloudy", "多云", "CPU 在轻微抬升，但还没到失控。", "CPU 在轻微抬升，但还没到失控", "轻压可监测", "#9CB7D8", "cloud"),
			Summary: "系统还没有失控，但有一个热点进程值得先看。",
			TopPriorities: []model.SituationPriority{
				{Title: "锁定热点进程", WhyNow: "CPU 占用已经往上抬", Prepared: "已把曲线压缩成候选项", Consequence: "继续放任会让桌面开始发烫"},
				{Title: "验证缓存刷新", WhyNow: "刚跑完一轮同步", Prepared: "回滚点已经保留", Consequence: "如果缓存错位，会连带影响后续状态"},
				{Title: "观察一轮趋势", WhyNow: "目前还没到强干预", Prepared: "已准备只读观察", Consequence: "过早重启可能会丢上下文"},
			},
			Prepared:    []string{"已把异常曲线压到 3 个候选", "已预留回滚点", "可先做只读观察"},
			HabitBasis:  "你处理资源异常时会先看是否只是短时波峰。",
			ActionLabel: "聚焦异常源",
			Pressure:    "多云",
		},
	}
}

func buildMemory(now time.Time) model.MemoryData {
	return model.MemoryData{
		Headline:   "这是按你的习惯判断的",
		Note:       "我不会把习惯当成命令，只把它当成优先级提示。",
		Confidence: "高",
		UpdatedAt:  now.Format("2006-01-02 15:04:05"),
		Habits: []model.MemoryHabit{
			{Scene: "日报整理", Cue: "这个时间点你通常会开始写日报", Basis: "你常在收尾时先把完成项列出来", Confidence: "高"},
			{Scene: "合同审阅", Cue: "你处理合同通常先看版本差异", Basis: "你更习惯先判断改动边界", Confidence: "高"},
			{Scene: "报错排查", Cue: "遇到类似报错时优先检查路径", Basis: "你的修复习惯偏路径和环境", Confidence: "高"},
		},
	}
}

func buildRiskPreviews() map[string]model.RiskPreview {
	levels := []model.AuthorizationLevel{
		{Key: "preview", Label: "仅预览", Description: "只生成预演和解释，不写入任何变化"},
		{Key: "draft", Label: "仅生成草稿", Description: "生成可检查的草稿或修复建议，不对外执行"},
		{Key: "execute_once", Label: "允许执行一次", Description: "按当前授权深度执行一次并记录回滚点"},
	}

	return map[string]model.RiskPreview{
		"generate_confirmation_draft": {
			SceneID:             "contract-review",
			SceneTitle:          "合同审阅",
			ActionKey:           "generate_confirmation_draft",
			ActionLabel:         "生成确认稿",
			RiskLevel:           "中高",
			Summary:             "这一步会影响外发版本，先做雷暴预演再决定是否发送。",
			Impacts:             []string{"会影响对外发送的版本", "审批记录会新增一次确认痕迹", "版本差异需要先标记为已确认"},
			Rollbacks:           []string{"确认稿可撤回", "未发送前可回到原版本", "草稿可继续修改"},
			Irreversible:        []string{"一旦外发，对方可能已经开始审核", "如果进入签字流程，回滚成本会上升"},
			AuthorizationLevels: levels,
			RecommendedDepth:    "draft",
		},
		"auto_fix": {
			SceneID:             "debug-triage",
			SceneTitle:          "报错排查",
			ActionKey:           "auto_fix",
			ActionLabel:         "自动修复建议",
			RiskLevel:           "高",
			Summary:             "自动修复会先改路径基准，风险来自写入与重试。",
			Impacts:             []string{"可能调整工作目录基准", "可能写入配置缓存", "可能触发一次本地重启"},
			Rollbacks:           []string{"只读验证可随时撤销", "路径修改可回到原值", "重启前可以取消"},
			Irreversible:        []string{"如果覆盖了原配置文件，需要重新生成", "若写入缓存后没有记录回滚点，就只能手动恢复"},
			AuthorizationLevels: levels,
			RecommendedDepth:    "preview",
		},
	}
}

func buildActionOutcomes() map[string]model.ActionOutcome {
	draft := dailyDraft()
	return map[string]model.ActionOutcome{
		"extract_summary": {
			Mode:    "inline-note",
			Message: "已先把这页的重点提炼出来，可以直接看结论。",
			Summary: "这页内容已经缩成 3 条结论。",
		},
		"advance_draft": {
			Mode:    "draft-open",
			Message: "已先把日报草稿起好，你可以直接查看。",
			Summary: "日报草稿已经铺开。",
			Draft:   &draft,
		},
		"generate_confirmation_draft": {
			Mode:          "risk-preview",
			Message:       "先别直接外发，雷暴预演已经准备好了。",
			RiskActionKey: "generate_confirmation_draft",
		},
		"auto_fix": {
			Mode:          "risk-preview",
			Message:       "自动修复建议已准备好，先选授权深度。",
			RiskActionKey: "auto_fix",
		},
		"inspect_metrics": {
			Mode:    "inline-note",
			Message: "已把异常源缩到 3 个候选项。",
			Summary: "资源异常已经收拢到可检查的范围。",
		},
	}
}

func buildLogs() []model.LogEntry {
	return []model.LogEntry{
		{Time: "09:12", Scene: "网页阅读", Action: "提炼重点", Result: "阅读纪要已生成", Rollback: "未修改原页面"},
		{Time: "10:03", Scene: "系统资源异常", Action: "聚焦异常源", Result: "异常曲线已缩到 3 个候选项", Rollback: "可退回只读观察"},
		{Time: "15:27", Scene: "合同审阅", Action: "生成确认稿", Result: "已进入雷暴预演", Rollback: "草稿可撤回"},
		{Time: "18:03", Scene: "日报整理", Action: "推进一步", Result: "日报草稿已起好", Rollback: "可回退为只读"},
		{Time: "19:11", Scene: "报错排查", Action: "自动修复建议", Result: "已要求授权深度", Rollback: "不会写入系统"},
	}
}

func weather(state, label, hint, bandText, tone, accent, texture string) model.Weather {
	return model.Weather{
		State:    state,
		Label:    label,
		Hint:     hint,
		BandText: bandText,
		Tone:     tone,
		Accent:   accent,
		Texture:  texture,
	}
}

func action(label, key, actionType, desc string) model.SceneAction {
	return model.SceneAction{Label: label, Key: key, Type: actionType, Description: desc}
}

func dailyDraft() model.DraftData {
	return model.DraftData{
		Title:  "日报草稿",
		Status: "已起草",
		Sections: []model.DraftSection{
			{Header: "今日完成", Bullets: []string{"完成页面原型主视觉", "接入本地 Go Mock 服务", "整理天气带文案"}},
			{Header: "待跟进", Bullets: []string{"补齐合同确认点", "回看路径修复建议", "确认雷暴预演层的最后文案"}},
			{Header: "明日推进", Bullets: []string{"把天气带展开态势图再微调", "完善风险授权深度选择", "再扫一轮资源异常场景"}},
		},
		Notes:    []string{"草稿先按你的习惯排好，后面只要补细节。"},
		NextStep: "可直接查看并补一版给对外发出的口径",
	}
}
