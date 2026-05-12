package execution

import (
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/languagepolicy"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
)

func buildPrompt(request Request, inputText string) string {
	intentName := effectiveIntentName(request.Intent)
	replyLanguage := preferredReplyLanguageForRequest(request, inputText)
	targetLanguage := defaultTargetLanguage(request.Intent, replyLanguage)

	instruction := "请先根据输入判断用户想要什么帮助；如果目标不明确，请明确指出需要用户补充处理方式，不要把内容误当成总结任务。"
	if replyLanguage == languagepolicy.ReplyLanguageEnglish {
		instruction = "First determine what help the user wants. If the goal is still unclear, say exactly what extra instruction is needed instead of guessing a summarize task."
	}
	switch intentName {
	case defaultAgentLoopIntentName:
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			instruction = "Act like a desktop agent for the following input. If the goal is clear, answer directly. If critical information is still missing, say exactly what the user should add."
		} else {
			instruction = "请像桌面 Agent 一样理解以下输入。如果目标清晰，直接给出结果；如果仍缺少关键信息，请明确指出需要补充什么。"
		}
	case "rewrite":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			instruction = "Rewrite the following content in clearer, ready-to-use English while preserving the original meaning."
		} else {
			instruction = "请保留原意并以更清晰、可直接使用的中文改写以下内容。"
		}
	case "translate":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			instruction = fmt.Sprintf("Translate the following content into %s and return only the translation.", targetLanguage)
		} else {
			instruction = fmt.Sprintf("请将以下内容翻译成%s，并直接输出翻译结果。", targetLanguage)
		}
	case "explain":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			instruction = "Explain the following content in concise English, focusing on the key point and conclusion."
		} else {
			instruction = "请用简洁中文解释以下内容，突出重点和结论。"
		}
	case "write_file":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			instruction = "Generate English document content that can be saved directly, using a clear title and sections."
		} else {
			instruction = "请根据以下输入生成一份可直接保存为文档的中文内容，使用清晰标题和小节。"
		}
	case "summarize":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			instruction = "Summarize the following content and return a clear English summary."
		} else {
			instruction = "请总结以下内容，输出结构清晰的中文摘要。"
		}
	}

	inputLabel := "输入内容:"
	if replyLanguage == languagepolicy.ReplyLanguageEnglish {
		inputLabel = "Input:"
	}
	return strings.TrimSpace(instruction) + "\n\n" + inputLabel + "\n" + strings.TrimSpace(inputText)
}

func fallbackOutput(request Request, inputText string) string {
	intentName := effectiveIntentName(request.Intent)
	replyLanguage := preferredReplyLanguageForRequest(request, inputText)
	normalized := normalizeWhitespace(inputText)
	if normalized == "" {
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			normalized = "No usable input"
		} else {
			normalized = presentation.Text(presentation.MessageFallbackNoInput, nil)
		}
	}

	switch intentName {
	case "":
		return clarificationFallbackText(replyLanguage)
	case defaultAgentLoopIntentName:
		return clarificationFallbackText(replyLanguage)
	case "rewrite":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			return "Rewrite result:\n" + normalized
		}
		return presentation.Text(presentation.MessageFallbackRewriteHeader, nil) + "\n" + normalized
	case "translate":
		targetLanguage := defaultTargetLanguage(request.Intent, replyLanguage)
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			return fmt.Sprintf("Translation result (fallback, target language: %s):\n%s", targetLanguage, normalized)
		}
		return presentation.Text(presentation.MessageFallbackTranslate, map[string]string{"target_language": targetLanguage}) + "\n" + normalized
	case "explain":
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			return "Explanation result:\n" + firstNonEmpty(firstSentence(normalized), normalized)
		}
		return presentation.Text(presentation.MessageFallbackExplainHeader, nil) + "\n" + firstNonEmpty(firstSentence(normalized), normalized)
	case "write_file":
		fallthrough
	case "summarize":
		highlights := extractHighlights(normalized, 3)
		if len(highlights) == 0 {
			if replyLanguage == languagepolicy.ReplyLanguageEnglish {
				return "Summary result:\n- Nothing available to summarize"
			}
			return presentation.Text(presentation.MessageFallbackSummarizeTitle, nil) + "\n" + presentation.Text(presentation.MessageFallbackSummarizeEmpty, nil)
		}

		lines := []string{presentation.Text(presentation.MessageFallbackSummarizeTitle, nil)}
		if replyLanguage == languagepolicy.ReplyLanguageEnglish {
			lines = []string{"Summary result:"}
		}
		for _, highlight := range highlights {
			lines = append(lines, "- "+highlight)
		}
		return strings.Join(lines, "\n")
	default:
		return normalized
	}
}

func defaultTargetLanguage(taskIntent map[string]any, replyLanguage string) string {
	targetLanguage := strings.TrimSpace(stringValue(mapValue(taskIntent, "arguments"), "target_language", ""))
	if targetLanguage != "" {
		return targetLanguage
	}
	if replyLanguage == languagepolicy.ReplyLanguageEnglish {
		return "Chinese"
	}
	return "中文"
}

func clarificationFallbackText(replyLanguage string) string {
	if replyLanguage == languagepolicy.ReplyLanguageEnglish {
		return "I am not sure how you want me to handle this yet. Please clarify your goal, for example explain, translate, rewrite, or summarize."
	}
	return presentation.Text(presentation.MessageFallbackClarify, nil)
}

// preferredReplyLanguageForRequest keeps language selection anchored to the raw
// user instruction before it considers the operated-on object text. This keeps
// English commands like "translate this" from falling back to Chinese just
// because the selected content itself is non-English.
func preferredReplyLanguageForRequest(request Request, inputText string) string {
	if trimmed := strings.TrimSpace(request.ReplyLanguage); trimmed != "" {
		return trimmed
	}
	for _, candidate := range []string{request.Snapshot.Text, request.Snapshot.ErrorText, request.Snapshot.SelectionText} {
		trimmed := strings.TrimSpace(candidate)
		if trimmed != "" {
			return languagepolicy.PreferredReplyLanguage(trimmed)
		}
	}
	return languagepolicy.PreferredReplyLanguage(inputText)
}

func effectiveIntentName(taskIntent map[string]any) string {
	return strings.TrimSpace(stringValue(taskIntent, "name", ""))
}
