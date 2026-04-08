package rpc

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

func (s *Server) registerHandlers() {
	s.handlers = map[string]methodHandler{
		"agent.input.submit":                   s.handleAgentInputSubmit,
		"agent.task.start":                     s.handleAgentTaskStart,
		"agent.task.confirm":                   s.handleAgentTaskConfirm,
		"agent.recommendation.get":             s.handleAgentRecommendationGet,
		"agent.recommendation.feedback.submit": s.handleAgentRecommendationFeedbackSubmit,
		"agent.task.list":                      s.handleAgentTaskList,
		"agent.task.detail.get":                s.handleAgentTaskDetailGet,
		"agent.task.control":                   s.handleAgentTaskControl,
		"agent.task_inspector.config.get":      s.handleAgentTaskInspectorConfigGet,
		"agent.task_inspector.config.update":   s.handleAgentTaskInspectorConfigUpdate,
		"agent.task_inspector.run":             s.handleAgentTaskInspectorRun,
		"agent.notepad.list":                   s.handleAgentNotepadList,
		"agent.notepad.convert_to_task":        s.handleAgentNotepadConvertToTask,
		"agent.dashboard.overview.get":         s.handleAgentDashboardOverviewGet,
		"agent.dashboard.module.get":           s.handleAgentDashboardModuleGet,
		"agent.mirror.overview.get":            s.handleAgentMirrorOverviewGet,
		"agent.security.summary.get":           s.handleAgentSecuritySummaryGet,
		"agent.security.pending.list":          s.handleAgentSecurityPendingList,
		"agent.security.respond":               s.handleAgentSecurityRespond,
		"agent.settings.get":                   s.handleAgentSettingsGet,
		"agent.settings.update":                s.handleAgentSettingsUpdate,
	}
}

func (s *Server) handleAgentInputSubmit(params map[string]any) (any, *rpcError) {
	input := mapValue(params, "input")
	inputMode := stringValue(input, "input_mode", "text")
	taskID := fmt.Sprintf("task_%d", s.now().UnixNano())
	title := fmt.Sprintf("处理输入：%s", truncateText(stringValue(input, "text", "未命名输入"), 16))
	sourceType := "hover_input"
	if inputMode == "voice" {
		sourceType = "voice"
	}

	return map[string]any{
		"task":           s.buildTask(taskID, title, sourceType, "confirming_intent", s.defaultIntent("summarize"), "intent_confirmation", "green", false),
		"bubble_message": s.buildBubbleMessage(taskID, "intent_confirm", "你是想让我先总结当前内容吗？"),
	}, nil
}

func (s *Server) handleAgentTaskStart(params map[string]any) (any, *rpcError) {
	input := mapValue(params, "input")
	intent := mapValue(params, "intent")
	trigger := stringValue(params, "trigger", "hover_text_input")
	taskID := fmt.Sprintf("task_%d", s.now().UnixNano())
	sourceType := taskSourceFromTrigger(trigger)
	title := s.taskTitleFromInput(input, intent)

	if len(intent) > 0 {
		return map[string]any{
			"task":            s.buildTask(taskID, title, sourceType, "completed", intent, "return_result", "green", true),
			"bubble_message":  s.buildBubbleMessage(taskID, "result", "结果已经生成，可直接查看。"),
			"delivery_result": s.buildDeliveryResult(taskID, "bubble", "解释结果", "结果已通过气泡返回"),
		}, nil
	}

	return map[string]any{
		"task":            s.buildTask(taskID, title, sourceType, "confirming_intent", s.defaultIntent("summarize"), "intent_confirmation", "green", false),
		"bubble_message":  s.buildBubbleMessage(taskID, "intent_confirm", "你是想让我按当前对象继续处理吗？"),
		"delivery_result": nil,
	}, nil
}

func (s *Server) handleAgentTaskConfirm(params map[string]any) (any, *rpcError) {
	taskID := stringValue(params, "task_id", "task_001")
	intent := mapValue(params, "corrected_intent")
	if len(intent) == 0 {
		intent = s.defaultIntent("summarize")
	}

	return map[string]any{
		"task":            s.buildTask(taskID, "已确认任务意图", "selected_text", "processing", intent, "generate_output", "green", false),
		"bubble_message":  s.buildBubbleMessage(taskID, "status", "已按确认后的意图开始处理。"),
		"delivery_result": nil,
	}, nil
}

func (s *Server) handleAgentRecommendationGet(params map[string]any) (any, *rpcError) {
	selectionText := stringValue(mapValue(params, "context"), "selection_text", "当前内容")
	return map[string]any{
		"cooldown_hit": false,
		"items": []map[string]any{
			{
				"recommendation_id": "rec_001",
				"text":              fmt.Sprintf("要不要我先总结这段内容：%s", truncateText(selectionText, 12)),
				"intent":            s.defaultIntent("summarize"),
			},
			{
				"recommendation_id": "rec_002",
				"text":              "也可以直接改写成更正式的版本。",
				"intent":            s.defaultIntent("rewrite"),
			},
		},
	}, nil
}

func (s *Server) handleAgentRecommendationFeedbackSubmit(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{"applied": true}, nil
}

func (s *Server) handleAgentTaskList(params map[string]any) (any, *rpcError) {
	group := stringValue(params, "group", "unfinished")
	taskID := "task_list_001"
	status := "processing"
	finished := false
	if group == "finished" {
		status = "completed"
		finished = true
	}

	return map[string]any{
		"items": []map[string]any{
			s.buildTask(taskID, "整理 Q3 复盘要点", "hover_input", status, s.defaultIntent("summarize"), "generate_summary", "green", finished),
		},
		"page": s.buildPage(intValue(params, "limit", 20), intValue(params, "offset", 0), 1),
	}, nil
}

func (s *Server) handleAgentTaskDetailGet(params map[string]any) (any, *rpcError) {
	taskID := stringValue(params, "task_id", "task_detail_001")
	return map[string]any{
		"task": s.buildTask(taskID, "整理 Q3 复盘要点", "hover_input", "processing", s.defaultIntent("summarize"), "generate_summary", "green", false),
		"timeline": []map[string]any{
			s.buildTaskStep(taskID, "step_001", "recognize_input_object", "completed", 1, "识别到拖入文件", "确认是文档总结任务"),
			s.buildTaskStep(taskID, "step_002", "generate_summary", "running", 2, "读取文档内容", "正在生成摘要"),
		},
		"artifacts": []map[string]any{
			s.buildArtifact(taskID, "generated_doc", "Q3复盘.md", "D:/CialloClawWorkspace/Q3复盘.md"),
		},
		"mirror_references": []map[string]any{
			s.buildMirrorReference(),
		},
		"security_summary": map[string]any{
			"security_status":        "normal",
			"risk_level":             "green",
			"pending_authorizations": 0,
			"latest_restore_point":   s.buildRecoveryPoint(taskID),
		},
	}, nil
}

func (s *Server) handleAgentTaskControl(params map[string]any) (any, *rpcError) {
	taskID := stringValue(params, "task_id", "task_control_001")
	action := stringValue(params, "action", "pause")
	status := map[string]string{
		"pause":   "paused",
		"resume":  "processing",
		"cancel":  "cancelled",
		"restart": "processing",
	}[action]
	if status == "" {
		status = "processing"
	}

	return map[string]any{
		"task":           s.buildTask(taskID, "受控任务", "hover_input", status, s.defaultIntent("summarize"), "control_applied", "green", action == "cancel"),
		"bubble_message": s.buildBubbleMessage(taskID, "status", fmt.Sprintf("任务控制动作已执行：%s", action)),
	}, nil
}

func (s *Server) handleAgentTaskInspectorConfigGet(params map[string]any) (any, *rpcError) {
	_ = params
	return s.buildInspectorConfig(), nil
}

func (s *Server) handleAgentTaskInspectorConfigUpdate(params map[string]any) (any, *rpcError) {
	return map[string]any{
		"updated":          true,
		"effective_config": s.buildInspectorConfigFromParams(params),
	}, nil
}

func (s *Server) handleAgentTaskInspectorRun(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{
		"inspection_id": "insp_001",
		"summary": map[string]any{
			"parsed_files":     3,
			"identified_items": 12,
			"due_today":        2,
			"overdue":          1,
			"stale":            3,
		},
		"suggestions": []string{"优先处理今天到期的复盘邮件", "下周评审材料建议先生成草稿"},
	}, nil
}

func (s *Server) handleAgentNotepadList(params map[string]any) (any, *rpcError) {
	group := stringValue(params, "group", "upcoming")
	itemStatus := "normal"
	if group == "upcoming" {
		itemStatus = "due_today"
	}

	return map[string]any{
		"items": []map[string]any{
			{
				"item_id":          "todo_001",
				"title":            "整理 Q3 复盘要点",
				"bucket":           group,
				"status":           itemStatus,
				"type":             "one_time",
				"due_at":           "2026-04-07T18:00:00+08:00",
				"agent_suggestion": "先生成一个 3 点摘要",
			},
		},
		"page": s.buildPage(intValue(params, "limit", 20), intValue(params, "offset", 0), 1),
	}, nil
}

func (s *Server) handleAgentNotepadConvertToTask(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{
		"task": s.buildTask("task_401", "整理 Q3 复盘要点", "todo", "confirming_intent", s.defaultIntent("summarize"), "intent_confirmation", "green", false),
	}, nil
}

func (s *Server) handleAgentDashboardOverviewGet(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{
		"overview": map[string]any{
			"focus_summary": map[string]any{
				"task_id":      "task_201",
				"title":        "整理 Q3 复盘要点",
				"status":       "processing",
				"current_step": "正在生成摘要",
				"next_action":  "等待用户查看结果",
				"updated_at":   s.nowRFC3339(),
			},
			"trust_summary": map[string]any{
				"risk_level":             "yellow",
				"pending_authorizations": 1,
				"has_restore_point":      true,
				"workspace_path":         "D:/CialloClawWorkspace",
			},
			"quick_actions":     []string{"打开任务详情", "查看待授权操作"},
			"global_state":      s.orchestrator.Snapshot(),
			"high_value_signal": []string{"最近一次任务命中了 2 条历史偏好"},
		},
	}, nil
}

func (s *Server) handleAgentDashboardModuleGet(params map[string]any) (any, *rpcError) {
	module := stringValue(params, "module", "mirror")
	tab := stringValue(params, "tab", "daily_summary")
	return map[string]any{
		"module": module,
		"tab":    tab,
		"summary": map[string]any{
			"completed_tasks":   3,
			"generated_outputs": 5,
			"authorizations":    1,
			"exceptions":        0,
		},
		"highlights": []string{"完成了 3 项内容整理任务", "生成了 1 份方案稿和 2 份摘要"},
	}, nil
}

func (s *Server) handleAgentMirrorOverviewGet(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{
		"history_summary": []string{"最近两周反复处理周报与复盘类任务", "更偏好简洁、可复用的输出格式"},
		"daily_summary": map[string]any{
			"date":              "2026-04-07",
			"completed_tasks":   3,
			"generated_outputs": 5,
		},
		"profile": map[string]any{
			"work_style":       "偏好结构化输出",
			"preferred_output": "3点摘要",
			"active_hours":     "10-12h",
		},
		"memory_references": []map[string]any{s.buildMirrorReference()},
	}, nil
}

func (s *Server) handleAgentSecuritySummaryGet(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{
		"summary": map[string]any{
			"security_status":        "pending_confirmation",
			"pending_authorizations": 1,
			"latest_restore_point":   s.buildRecoveryPoint("task_301"),
			"token_cost_summary":     s.buildTokenCostSummary(),
		},
	}, nil
}

func (s *Server) handleAgentSecurityPendingList(params map[string]any) (any, *rpcError) {
	return map[string]any{
		"items": []map[string]any{s.buildApprovalRequest("task_301", "appr_001")},
		"page":  s.buildPage(intValue(params, "limit", 20), intValue(params, "offset", 0), 1),
	}, nil
}

func (s *Server) handleAgentSecurityRespond(params map[string]any) (any, *rpcError) {
	taskID := stringValue(params, "task_id", "task_301")
	approvalID := stringValue(params, "approval_id", "appr_001")
	decision := stringValue(params, "decision", "allow_once")
	rememberRule := boolValue(params, "remember_rule", false)

	return map[string]any{
		"authorization_record": s.buildAuthorizationRecord(taskID, approvalID, decision, rememberRule),
		"task":                 s.buildTask(taskID, "高风险任务继续执行", "dragged_file", "processing", s.defaultIntent("write_file"), "resume_execution", "red", false),
		"bubble_message":       s.buildBubbleMessage(taskID, "status", "已允许本次操作，任务继续执行。"),
		"impact_scope":         s.buildImpactScope(),
	}, nil
}

func (s *Server) handleAgentSettingsGet(params map[string]any) (any, *rpcError) {
	_ = params
	return map[string]any{
		"settings": s.buildSettingsSnapshot()["settings"],
	}, nil
}

func (s *Server) handleAgentSettingsUpdate(params map[string]any) (any, *rpcError) {
	updatedKeys := make([]string, 0)
	effectiveSettings := map[string]any{}
	applyMode := "immediate"
	needRestart := false

	sections := []string{"general", "floating_ball", "memory", "task_automation", "data_log"}
	for _, section := range sections {
		sectionValue := mapValue(params, section)
		if len(sectionValue) == 0 {
			continue
		}

		effectiveSettings[section] = sectionValue
		keys := make([]string, 0, len(sectionValue))
		for key := range sectionValue {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			updatedKeys = append(updatedKeys, fmt.Sprintf("%s.%s", section, key))
		}

		if section == "general" {
			if _, ok := sectionValue["language"]; ok {
				applyMode = "restart_required"
				needRestart = true
			}
		}
	}

	return map[string]any{
		"updated_keys":       updatedKeys,
		"effective_settings": effectiveSettings,
		"apply_mode":         applyMode,
		"need_restart":       needRestart,
	}, nil
}

func (s *Server) buildTask(taskID, title, sourceType, status string, intent map[string]any, currentStep, riskLevel string, finished bool) map[string]any {
	now := s.nowRFC3339()
	result := map[string]any{
		"task_id":      taskID,
		"title":        title,
		"source_type":  sourceType,
		"status":       status,
		"intent":       intent,
		"current_step": currentStep,
		"risk_level":   riskLevel,
		"started_at":   now,
		"updated_at":   now,
		"finished_at":  nil,
	}

	if finished {
		result["finished_at"] = now
	}

	return result
}

func (s *Server) buildTaskStep(taskID, stepID, name, status string, orderIndex int, inputSummary, outputSummary string) map[string]any {
	return map[string]any{
		"step_id":        stepID,
		"task_id":        taskID,
		"name":           name,
		"status":         status,
		"order_index":    orderIndex,
		"input_summary":  inputSummary,
		"output_summary": outputSummary,
	}
}

func (s *Server) buildBubbleMessage(taskID, bubbleType, text string) map[string]any {
	return map[string]any{
		"bubble_id":  fmt.Sprintf("bubble_%d", s.now().UnixNano()),
		"task_id":    taskID,
		"type":       bubbleType,
		"text":       text,
		"pinned":     false,
		"hidden":     false,
		"created_at": s.nowRFC3339(),
	}
}

func (s *Server) buildDeliveryResult(taskID, deliveryType, title, previewText string) map[string]any {
	return map[string]any{
		"type":  deliveryType,
		"title": title,
		"payload": map[string]any{
			"path":    nil,
			"url":     nil,
			"task_id": taskID,
		},
		"preview_text": previewText,
	}
}

func (s *Server) buildArtifact(taskID, artifactType, title, path string) map[string]any {
	return map[string]any{
		"artifact_id":   fmt.Sprintf("art_%d", s.now().UnixNano()),
		"task_id":       taskID,
		"artifact_type": artifactType,
		"title":         title,
		"path":          path,
		"mime_type":     "text/markdown",
	}
}

func (s *Server) buildApprovalRequest(taskID, approvalID string) map[string]any {
	return map[string]any{
		"approval_id":    approvalID,
		"task_id":        taskID,
		"operation_name": "write_file",
		"risk_level":     "red",
		"target_object":  "C:/Users/demo/Desktop/report.docx",
		"reason":         "out_of_workspace",
		"status":         "pending",
		"created_at":     s.nowRFC3339(),
	}
}

func (s *Server) buildAuthorizationRecord(taskID, approvalID, decision string, rememberRule bool) map[string]any {
	return map[string]any{
		"authorization_record_id": fmt.Sprintf("auth_%d", s.now().UnixNano()),
		"task_id":                 taskID,
		"approval_id":             approvalID,
		"decision":                decision,
		"remember_rule":           rememberRule,
		"operator":                "user",
		"created_at":              s.nowRFC3339(),
	}
}

func (s *Server) buildRecoveryPoint(taskID string) map[string]any {
	return map[string]any{
		"recovery_point_id": fmt.Sprintf("rp_%d", s.now().UnixNano()),
		"task_id":           taskID,
		"summary":           "工具执行前恢复点",
		"created_at":        s.nowRFC3339(),
		"objects":           []string{"D:/CialloClawWorkspace/temp.md"},
	}
}

func (s *Server) buildImpactScope() map[string]any {
	return map[string]any{
		"files":                    []string{"D:/CialloClawWorkspace/report.md"},
		"webpages":                 []string{},
		"apps":                     []string{},
		"out_of_workspace":         false,
		"overwrite_or_delete_risk": false,
	}
}

func (s *Server) buildMirrorReference() map[string]any {
	return map[string]any{
		"memory_id": "pref_001",
		"reason":    "当前任务命中了用户的输出偏好",
		"summary":   "偏好简洁三点式摘要",
	}
}

func (s *Server) buildTokenCostSummary() map[string]any {
	return map[string]any{
		"current_task_tokens":   2847,
		"current_task_cost":     0.12,
		"today_tokens":          9321,
		"today_cost":            0.46,
		"single_task_limit":     10.0,
		"daily_limit":           50.0,
		"budget_auto_downgrade": true,
	}
}

func (s *Server) buildInspectorConfig() map[string]any {
	return map[string]any{
		"task_sources":           []string{"D:/workspace/todos"},
		"inspection_interval":    map[string]any{"unit": "minute", "value": 15},
		"inspect_on_file_change": true,
		"inspect_on_startup":     true,
		"remind_before_deadline": true,
		"remind_when_stale":      false,
	}
}

func (s *Server) buildInspectorConfigFromParams(params map[string]any) map[string]any {
	config := s.buildInspectorConfig()
	if taskSources := stringSliceValue(params["task_sources"]); len(taskSources) > 0 {
		config["task_sources"] = taskSources
	}
	if interval := mapValue(params, "inspection_interval"); len(interval) > 0 {
		config["inspection_interval"] = map[string]any{
			"unit":  stringValue(interval, "unit", "minute"),
			"value": intValue(interval, "value", 15),
		}
	}
	config["inspect_on_file_change"] = boolValue(params, "inspect_on_file_change", true)
	config["inspect_on_startup"] = boolValue(params, "inspect_on_startup", true)
	config["remind_before_deadline"] = boolValue(params, "remind_before_deadline", true)
	config["remind_when_stale"] = boolValue(params, "remind_when_stale", false)
	return config
}

func (s *Server) buildSettingsSnapshot() map[string]any {
	return map[string]any{
		"settings": map[string]any{
			"general": map[string]any{
				"language":                   "zh-CN",
				"auto_launch":                true,
				"theme_mode":                 "follow_system",
				"voice_notification_enabled": true,
				"voice_type":                 "default_female",
				"download": map[string]any{
					"workspace_path":            "D:/CialloClawWorkspace",
					"ask_before_save_each_file": true,
				},
			},
			"floating_ball": map[string]any{
				"auto_snap":        true,
				"idle_translucent": true,
				"position_mode":    "draggable",
				"size":             "medium",
			},
			"memory": map[string]any{
				"enabled":                  true,
				"lifecycle":                "30d",
				"work_summary_interval":    map[string]any{"unit": "day", "value": 7},
				"profile_refresh_interval": map[string]any{"unit": "week", "value": 2},
			},
			"task_automation": map[string]any{
				"inspect_on_startup":     true,
				"inspect_on_file_change": true,
				"inspection_interval":    map[string]any{"unit": "minute", "value": 15},
				"task_sources":           []string{"D:/workspace/todos"},
				"remind_before_deadline": true,
				"remind_when_stale":      false,
			},
			"data_log": map[string]any{
				"provider":              "openai",
				"budget_auto_downgrade": true,
			},
		},
	}
}

func (s *Server) buildPage(limit, offset, total int) map[string]any {
	return map[string]any{
		"limit":    limit,
		"offset":   offset,
		"total":    total,
		"has_more": offset+limit < total,
	}
}

func (s *Server) taskTitleFromInput(input map[string]any, intent map[string]any) string {
	if len(intent) > 0 {
		intentName := stringValue(intent, "name", "处理内容")
		return fmt.Sprintf("%s当前对象", strings.Title(intentName))
	}

	if text := stringValue(input, "text", ""); text != "" {
		return fmt.Sprintf("处理输入：%s", truncateText(text, 16))
	}

	if files := stringSliceValue(input["files"]); len(files) > 0 {
		return "整理并总结拖入文件"
	}

	if stringValue(input, "error_message", "") != "" {
		return "处理当前错误信息"
	}

	return "处理当前任务对象"
}

func (s *Server) defaultIntent(name string) map[string]any {
	arguments := map[string]any{}
	if name == "summarize" {
		arguments["style"] = "key_points"
	}
	if name == "rewrite" {
		arguments["tone"] = "professional"
	}
	return map[string]any{
		"name":      name,
		"arguments": arguments,
	}
}

func (s *Server) nowRFC3339() string {
	return s.now().Format(time.RFC3339)
}

func taskSourceFromTrigger(trigger string) string {
	sources := map[string]string{
		"voice_commit":         "voice",
		"hover_text_input":     "hover_input",
		"text_selected_click":  "selected_text",
		"file_drop":            "dragged_file",
		"error_detected":       "error_signal",
		"recommendation_click": "hover_input",
	}

	if sourceType, ok := sources[trigger]; ok {
		return sourceType
	}

	return "hover_input"
}

func truncateText(value string, maxLength int) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= maxLength {
		return trimmed
	}

	return trimmed[:maxLength] + "..."
}
