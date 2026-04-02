const assert = require("assert");
const {
  MEETING_ALL_EMPTY_TEXT,
  MEETING_SOURCE_EMPTY_TEXT,
  splitTranscriptBlocksBySource,
  buildSourceTranscriptText,
  parseStructuredSummaryResponse,
  trimCaptionItems
} = require("../src/shared/meeting-summary-utils");

function run() {
  const now = Date.now();
  const blocks = [
    {
      id: "system-1",
      source: "system",
      time: "10:00",
      capturedAt: now - 20_000,
      text: "对方确认本周五前交付接口联调。"
    },
    {
      id: "microphone-1",
      source: "microphone",
      time: "10:01",
      capturedAt: now - 10_000,
      text: "我这边会先把验收字段再确认一遍。"
    },
    {
      id: "expired",
      source: "system",
      time: "09:40",
      capturedAt: now - 190_000,
      text: "这条已经过期，不该进入最近两分钟。"
    }
  ];

  const grouped = splitTranscriptBlocksBySource(blocks, now, { windowSeconds: 120 });
  assert.equal(grouped.recentBlocks.length, 2);
  assert.equal(grouped.recentSystemBlocks.length, 1);
  assert.equal(grouped.recentMicrophoneBlocks.length, 1);
  assert.ok(buildSourceTranscriptText(grouped.recentSystemBlocks, "系统音频").includes("交付接口联调"));

  const parsedJson = parseStructuredSummaryResponse("{\"othersText\":\"他人确认本周五前完成联调。\",\"selfText\":\"我表示会复核验收字段。\"}");
  assert.equal(parsedJson.othersText, "他人确认本周五前完成联调。");
  assert.equal(parsedJson.selfText, "我表示会复核验收字段。");

  const parsedFallback = parseStructuredSummaryResponse("他人讲话：对方在确认交付时间。\n我讲话：我补充了自己的跟进动作。");
  assert.equal(parsedFallback.othersText, "对方在确认交付时间。");
  assert.equal(parsedFallback.selfText, "我补充了自己的跟进动作。");

  const trimmedItems = trimCaptionItems([
    {
      id: "keep",
      text: "这一条保留",
      time: "10:02",
      capturedAt: now - 15_000
    },
    {
      id: "drop",
      text: "这一条过期",
      time: "09:55",
      capturedAt: now - 220_000
    }
  ], now, {
    historyMs: 180_000,
    maxItems: 6
  });
  assert.equal(trimmedItems.length, 1);
  assert.equal(trimmedItems[0].id, "keep");

  process.stdout.write(JSON.stringify({
    ok: true,
    parsedJson,
    parsedFallback,
    captionPlaceholder: MEETING_SOURCE_EMPTY_TEXT,
    allEmptyPlaceholder: MEETING_ALL_EMPTY_TEXT
  }, null, 2));
}

run();
