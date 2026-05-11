import type {
  AgentRecommendationGetParams,
  AgentRecommendationGetResult,
  IntentPayload,
  RecommendationItem,
} from "@cialloclaw/protocol";

type MockRecommendationEntry = {
  text: string;
  intentName: string;
  intentArgs?: Record<string, unknown>;
};

const MOCK_RECOMMENDATIONS_BY_SCENE: Record<string, MockRecommendationEntry[]> = {
  idle: [
    {
      text: "看看当前窗口截图",
      intentName: "screenshot_analyze",
      intentArgs: {},
    },
    {
      text: "总结当前页面的主要内容",
      intentName: "summarize_page",
      intentArgs: {},
    },
  ],
  hover: [
    {
      text: "帮我翻译这段内容",
      intentName: "translate",
      intentArgs: {},
    },
    {
      text: "解释这段代码",
      intentName: "explain_code",
      intentArgs: {},
    },
  ],
  error: [
    {
      text: "帮我排查这个错误",
      intentName: "debug_error",
      intentArgs: {},
    },
    {
      text: "修复这个报错",
      intentName: "fix_error",
      intentArgs: {},
    },
  ],
};

function buildMockRecommendationItem(entry: MockRecommendationEntry, index: number): RecommendationItem {
  return {
    recommendation_id: `mock-rec-${Date.now()}-${index}`,
    text: entry.text,
    intent: {
      name: entry.intentName,
      arguments: entry.intentArgs ?? {},
    },
  };
}

export async function getMockRecommendations(
  params: AgentRecommendationGetParams,
): Promise<AgentRecommendationGetResult> {
  const scene = params.scene ?? "idle";
  const sceneRecommendations = MOCK_RECOMMENDATIONS_BY_SCENE[scene] ?? MOCK_RECOMMENDATIONS_BY_SCENE.idle;

  return {
    cooldown_hit: false,
    items: sceneRecommendations.map((entry, index) => buildMockRecommendationItem(entry, index)),
  };
}