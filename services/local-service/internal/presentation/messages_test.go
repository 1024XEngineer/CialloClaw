package presentation

import (
	"strings"
	"testing"
)

func TestRenderInterpolatesSemanticMessage(t *testing.T) {
	got := Render("zh-CN", Message{
		Key:    MessageTaskTitleScreenFallback,
		Params: map[string]string{"subject": "Build Dashboard"},
	})
	if got != "处理：Build Dashboard" {
		t.Fatalf("expected interpolated title copy, got %q", got)
	}
}

func TestRenderInterpolatesOptionalProviderDetail(t *testing.T) {
	got := Text(MessageExecutionFailureAuth, DetailParam("missing scope"))
	if got != "执行失败：模型鉴权失败（missing scope），请检查 API Key 或访问权限。" {
		t.Fatalf("expected provider detail to be rendered, got %q", got)
	}

	redacted := Text(MessageExecutionFailureAuth, DetailParam(""))
	if redacted != "执行失败：模型鉴权失败，请检查 API Key 或访问权限。" {
		t.Fatalf("expected empty provider detail to be omitted, got %q", redacted)
	}
}

func TestRenderDoesNotReinterpolatePlaceholderTokensInsideValues(t *testing.T) {
	got := Text(MessageToolBubbleSearchMatches, map[string]string{
		"query": "\"first line\\n{count}\"",
		"count": "3",
	})
	if got != "页面搜索完成，关键词 \"first line\\n{count}\" 共匹配 3 处。" {
		t.Fatalf("expected value placeholders to remain literal, got %q", got)
	}
	if strings.Contains(got, "first line\\n3") {
		t.Fatalf("expected placeholder token inside query to stay untouched, got %q", got)
	}
}

func TestResultSpecMessagesExposeSemanticKeys(t *testing.T) {
	spec := ResultSpecMessagesForIntent("browser_snapshot")
	if spec.Title.Key != MessageResultTitleBrowserSnap {
		t.Fatalf("expected browser snapshot title key, got %s", spec.Title.Key)
	}
	if spec.Preview.Key != MessagePreviewBubble {
		t.Fatalf("expected bubble preview key, got %s", spec.Preview.Key)
	}
	if spec.BubbleText.Key != MessageBubbleResultBrowserSnapshot {
		t.Fatalf("expected browser snapshot bubble key, got %s", spec.BubbleText.Key)
	}
}

func TestApprovalResultSpecKeepsAuthorizationSpecificSummaryCopy(t *testing.T) {
	spec := RenderApprovalResultSpec("summarize")
	if spec.Title != "总结结果" {
		t.Fatalf("expected approval summary title copy, got %q", spec.Title)
	}

	defaultSpec := RenderResultSpec("summarize")
	if defaultSpec.Title != "处理结果" {
		t.Fatalf("expected default summary title copy to preserve task execution behavior, got %q", defaultSpec.Title)
	}
}

func TestTaskTitleMessageSelectsSemanticKeys(t *testing.T) {
	cases := []struct {
		name       string
		intentName string
		options    TaskTitleOptions
		want       MessageKey
	}{
		{name: "confirm", want: MessageTaskTitleConfirm},
		{name: "file summary", intentName: "summarize", options: TaskTitleOptions{IsFile: true}, want: MessageTaskTitleSummarizeFile},
		{name: "screen error", intentName: "screen_analyze", options: TaskTitleOptions{HasError: true}, want: MessageTaskTitleScreenError},
		{name: "generic", intentName: "unknown", want: MessageTaskTitleGeneric},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			message := TaskTitleMessage(tc.intentName, tc.options)
			if message.Key != tc.want {
				t.Fatalf("expected key %s, got %s", tc.want, message.Key)
			}
		})
	}
}

func TestTaskTitlePrefixesIncludeLegacyScreenTitles(t *testing.T) {
	prefixes := TaskTitlePrefixes()
	for _, expected := range []string{"查看屏幕：", "查看当前屏幕：", "查看屏幕报错："} {
		found := false
		for _, prefix := range prefixes {
			if prefix == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected title prefixes to include %q, got %v", expected, prefixes)
		}
	}
}
