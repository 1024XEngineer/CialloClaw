"use strict";

const MEETING_SUMMARY_WINDOW_SECONDS = 120;
const MEETING_CAPTION_HISTORY_MS = 3 * 60 * 1000;
const MEETING_CAPTION_MAX_ITEMS = 36;
const MEETING_SOURCE_EMPTY_TEXT = "本轮未识别到有效内容";
const MEETING_ALL_EMPTY_TEXT = "最近两分钟暂无可总结内容";

function normalizeText(value) {
  return typeof value === "string" ? value.trim() : "";
}

function createEmptyStructuredSummary(overrides = {}) {
  return {
    updatedAt: "",
    othersText: "",
    selfText: "",
    windowSeconds: MEETING_SUMMARY_WINDOW_SECONDS,
    isEmpty: false,
    note: "",
    ...overrides
  };
}

function normalizeStructuredSummary(input = {}) {
  const source = input && typeof input === "object" ? input : {};
  const parsedWindowSeconds = Number(source.windowSeconds);

  return createEmptyStructuredSummary({
    updatedAt: normalizeText(source.updatedAt),
    othersText: normalizeText(source.othersText),
    selfText: normalizeText(source.selfText),
    windowSeconds: Number.isFinite(parsedWindowSeconds) && parsedWindowSeconds > 0
      ? Math.round(parsedWindowSeconds)
      : MEETING_SUMMARY_WINDOW_SECONDS,
    isEmpty: Boolean(source.isEmpty),
    note: normalizeText(source.note)
  });
}

function hasStructuredSummaryContent(summary) {
  const normalized = normalizeStructuredSummary(summary);
  return Boolean(normalized.othersText || normalized.selfText || normalized.note);
}

function trimCaptionItems(items, now = Date.now(), options = {}) {
  const historyMs = Number.isFinite(options.historyMs) ? options.historyMs : MEETING_CAPTION_HISTORY_MS;
  const maxItems = Number.isFinite(options.maxItems) ? options.maxItems : MEETING_CAPTION_MAX_ITEMS;

  return (Array.isArray(items) ? items : [])
    .filter((item) => normalizeText(item?.text))
    .filter((item) => {
      const capturedAt = Number(item?.capturedAt || 0);
      return !capturedAt || (now - capturedAt) <= historyMs;
    })
    .slice(0, maxItems)
    .map((item) => ({
      ...item,
      text: normalizeText(item.text)
    }));
}

function splitTranscriptBlocksBySource(blocks, now = Date.now(), options = {}) {
  const windowSeconds = Number.isFinite(options.windowSeconds)
    ? options.windowSeconds
    : MEETING_SUMMARY_WINDOW_SECONDS;
  const windowMs = windowSeconds * 1000;
  const recentBlocks = (Array.isArray(blocks) ? blocks : [])
    .filter((entry) => normalizeText(entry?.text))
    .filter((entry) => {
      const capturedAt = Number(entry?.capturedAt || 0);
      return !capturedAt || (now - capturedAt) <= windowMs;
    })
    .sort((left, right) => Number(left?.capturedAt || 0) - Number(right?.capturedAt || 0));

  const recentSystemBlocks = recentBlocks.filter((entry) => entry.source !== "microphone");
  const recentMicrophoneBlocks = recentBlocks.filter((entry) => entry.source === "microphone");

  return {
    windowSeconds,
    recentBlocks,
    recentSystemBlocks,
    recentMicrophoneBlocks,
    hasAny: recentBlocks.length > 0,
    hasOthers: recentSystemBlocks.length > 0,
    hasSelf: recentMicrophoneBlocks.length > 0
  };
}

function buildSourceTranscriptText(blocks, label) {
  return (Array.isArray(blocks) ? blocks : [])
    .filter((entry) => normalizeText(entry?.text))
    .map((entry) => {
      const time = normalizeText(entry.time);
      return `[${label}${time ? ` ${time}` : ""}] ${normalizeText(entry.text)}`;
    })
    .join("\n");
}

function stripCodeFences(value) {
  const text = normalizeText(value);
  if (!text.startsWith("```")) {
    return text;
  }

  return text
    .replace(/^```(?:json)?/i, "")
    .replace(/```$/i, "")
    .trim();
}

function parseJsonCandidate(text) {
  const normalized = stripCodeFences(text);
  if (!normalized) {
    return null;
  }

  try {
    return JSON.parse(normalized);
  } catch (_error) {
    // noop
  }

  const startIndex = normalized.indexOf("{");
  const endIndex = normalized.lastIndexOf("}");
  if (startIndex === -1 || endIndex === -1 || endIndex <= startIndex) {
    return null;
  }

  try {
    return JSON.parse(normalized.slice(startIndex, endIndex + 1));
  } catch (_error) {
    return null;
  }
}

function parseStructuredSummaryResponse(rawText) {
  const text = normalizeText(rawText);
  if (!text) {
    return createEmptyStructuredSummary();
  }

  const jsonCandidate = parseJsonCandidate(text);
  if (jsonCandidate) {
    return normalizeStructuredSummary(jsonCandidate);
  }

  const othersMatch = text.match(/(?:他人讲话|他人|系统音频)\s*[:：]\s*([\s\S]*?)(?=(?:我讲话|我方|麦克风|自己发言)\s*[:：]|$)/i);
  const selfMatch = text.match(/(?:我讲话|我方|麦克风|自己发言)\s*[:：]\s*([\s\S]*)$/i);

  return normalizeStructuredSummary({
    othersText: othersMatch ? othersMatch[1] : "",
    selfText: selfMatch ? selfMatch[1] : ""
  });
}

function createTranscriptSnippet(transcript, fallback = MEETING_SOURCE_EMPTY_TEXT, maxLength = 96) {
  const normalized = normalizeText(transcript).replace(/\s+/g, " ");
  if (!normalized) {
    return fallback;
  }
  return normalized.length > maxLength ? `${normalized.slice(0, maxLength).trim()}...` : normalized;
}

module.exports = {
  MEETING_SUMMARY_WINDOW_SECONDS,
  MEETING_CAPTION_HISTORY_MS,
  MEETING_CAPTION_MAX_ITEMS,
  MEETING_SOURCE_EMPTY_TEXT,
  MEETING_ALL_EMPTY_TEXT,
  createEmptyStructuredSummary,
  normalizeStructuredSummary,
  hasStructuredSummaryContent,
  trimCaptionItems,
  splitTranscriptBlocksBySource,
  buildSourceTranscriptText,
  parseStructuredSummaryResponse,
  createTranscriptSnippet
};
