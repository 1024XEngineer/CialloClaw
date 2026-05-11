import type {
  AgentRecommendationGetParams,
  AgentRecommendationGetResult,
  RecommendationItem,
  RecommendationScene,
} from "@cialloclaw/protocol";

type MockRecommendationPriority = "high" | "medium" | "low";

type MockRecommendationTemplate = {
  text: string;
  label: string;
  intentName: string;
  intentArgs?: Record<string, unknown>;
  priority: MockRecommendationPriority;
};

type ExtendedRecommendationContext = AgentRecommendationGetParams["context"] & {
  clipboard_text?: string;
  error_text?: string;
  last_action?: string;
  dwell_millis?: number;
  copy_count?: number;
  window_switch_count?: number;
  page_switch_count?: number;
  selection?: {
    text?: string;
  };
  clipboard?: {
    text?: string;
  };
  error?: {
    message?: string;
  };
  behavior?: {
    last_action?: string;
    dwell_millis?: number;
    copy_count?: number;
    window_switch_count?: number;
    page_switch_count?: number;
  };
};

export type MockRecommendationPresentation = {
  label: string;
  copy: string;
  summary: string;
  reason: string;
  priority: MockRecommendationPriority;
  priorityLabel: string;
};

const MOCK_RECOMMENDATION_LIBRARY: Record<RecommendationScene, MockRecommendationTemplate[]> = {
  idle: [
    {
      text: "先看当前窗口",
      label: "看当前窗口",
      intentName: "summarize_page",
      priority: "high",
    },
    {
      text: "先帮我总结这一页",
      label: "整理这一页",
      intentName: "summarize_page",
      priority: "medium",
    },
    {
      text: "直接给我一个下一步",
      label: "给我下一步",
      intentName: "suggest_next_step",
      priority: "low",
    },
  ],
  hover: [
    {
      text: "解释一下这段内容",
      label: "解释这段",
      intentName: "explain",
      priority: "high",
    },
    {
      text: "把这段话改自然一点",
      label: "润色表达",
      intentName: "rewrite",
      priority: "medium",
    },
    {
      text: "整理成下一步",
      label: "整理下一步",
      intentName: "suggest_next_step",
      priority: "low",
    },
  ],
  selected_text: [
    {
      text: "先翻译这段",
      label: "翻译选区",
      intentName: "translate",
      priority: "high",
    },
    {
      text: "先解释选中的内容",
      label: "解释选区",
      intentName: "explain",
      priority: "medium",
    },
    {
      text: "整理成下一步",
      label: "整理下一步",
      intentName: "suggest_next_step",
      priority: "low",
    },
  ],
  error: [
    {
      text: "先分析这个错误",
      label: "分析错误",
      intentName: "debug_error",
      priority: "high",
    },
    {
      text: "给我一个修复方向",
      label: "修复方向",
      intentName: "fix_error",
      priority: "medium",
    },
  ],
};

function normalizeRecommendationText(value: string | undefined, limit = 48) {
  const trimmed = value?.trim() ?? "";

  if (trimmed === "") {
    return "";
  }

  return trimmed.length > limit ? `${trimmed.slice(0, limit - 1)}...` : trimmed;
}

function normalizeRecommendationCopy(value: string) {
  return value.replace(/\s+/g, " ").trim();
}

function getRecommendationContextSnapshot(context: ExtendedRecommendationContext) {
  return {
    appName: normalizeRecommendationText(context.app_name, 24),
    clipboardText: normalizeRecommendationText(context.clipboard_text ?? context.clipboard?.text, 44),
    copyCount: context.copy_count ?? context.behavior?.copy_count,
    dwellMillis: context.dwell_millis ?? context.behavior?.dwell_millis,
    errorText: normalizeRecommendationText(context.error_text ?? context.error?.message, 44),
    lastAction: normalizeRecommendationText(context.last_action ?? context.behavior?.last_action, 24),
    pageSwitchCount: context.page_switch_count ?? context.behavior?.page_switch_count,
    pageTitle: normalizeRecommendationText(context.page_title, 40),
    selectionText: normalizeRecommendationText(context.selection_text ?? context.selection?.text, 44),
    windowSwitchCount: context.window_switch_count ?? context.behavior?.window_switch_count,
  };
}

function getRecommendationTheme(input: {
  scene: RecommendationScene;
  context: ExtendedRecommendationContext;
}) {
  const snapshot = getRecommendationContextSnapshot(input.context);

  if (input.scene === "error" || snapshot.errorText !== "") {
    return { kind: "error" as const, snippet: snapshot.errorText };
  }

  if (input.scene === "selected_text" || snapshot.selectionText !== "") {
    return { kind: "selection" as const, snippet: snapshot.selectionText };
  }

  if (input.scene === "hover") {
    return { kind: "hover" as const, snippet: snapshot.pageTitle || snapshot.appName };
  }

  if (snapshot.clipboardText !== "") {
    return { kind: "clipboard" as const, snippet: snapshot.clipboardText };
  }

  return { kind: "page" as const, snippet: snapshot.pageTitle || snapshot.appName || "当前窗口" };
}

function resolveRecommendationPriorityLabel(priority: MockRecommendationPriority) {
  switch (priority) {
    case "high":
      return "优先推荐";
    case "medium":
      return "快速建议";
    case "low":
      return "补充建议";
    default:
      return "推荐";
  }
}

function resolveRecommendationSummary(input: {
  entry: MockRecommendationTemplate;
  scene: RecommendationScene;
  context: ExtendedRecommendationContext;
}) {
  const snapshot = getRecommendationContextSnapshot(input.context);
  const theme = getRecommendationTheme(input);

  switch (theme.kind) {
    case "error":
      return "我看到一个错误信号。";
    case "selection":
      return "我已经识别到你选中的内容了。";
    case "hover":
      return "我正在跟着你当前停留的位置看。";
    case "clipboard":
      return "我读到了剪贴板里的内容。";
    case "page":
    default:
      if (snapshot.lastAction !== "" && input.scene === "idle") {
        return "我先看你刚点开的入口。";
      }
      return "我先看当前窗口。";
  }
}

function resolveRecommendationReason(input: {
  entry: MockRecommendationTemplate;
  scene: RecommendationScene;
  context: ExtendedRecommendationContext;
}) {
  const snapshot = getRecommendationContextSnapshot(input.context);
  const theme = getRecommendationTheme(input);

  switch (theme.kind) {
    case "error":
      return "先把问题收窄，再决定是修复还是回退。";
    case "selection":
      return "这样可以直接从这段内容切入，最快。";
    case "hover":
      return "我先给你一个贴近当前上下文的建议。";
    case "clipboard":
      return "这样你不用重新找内容，能直接继续。";
    case "page":
    default:
      if (snapshot.lastAction !== "") {
        return "我会给你一个低成本的下一步。";
      }
      return input.entry.priority === "high" ? "先从最容易接上的动作开始。" : "我先给你一个轻量建议。";
  }
}

function createRecommendationContextSignature(scene: RecommendationScene, context: ExtendedRecommendationContext) {
  const snapshot = getRecommendationContextSnapshot(context);

  return [
    scene,
    snapshot.appName,
    snapshot.pageTitle,
    snapshot.selectionText,
    snapshot.clipboardText,
    snapshot.errorText,
    snapshot.lastAction,
    snapshot.dwellMillis ?? "",
    snapshot.copyCount ?? "",
    snapshot.windowSwitchCount ?? "",
    snapshot.pageSwitchCount ?? "",
  ].join("|");
}

function createStableRecommendationHash(value: string) {
  let hash = 0;

  for (let index = 0; index < value.length; index += 1) {
    hash = Math.imul(31, hash) + value.charCodeAt(index);
    hash |= 0;
  }

  return Math.abs(hash).toString(36);
}

function buildMockRecommendationItem(
  entry: MockRecommendationTemplate,
  index: number,
  scene: RecommendationScene,
  context: ExtendedRecommendationContext,
): RecommendationItem {
  const signature = createRecommendationContextSignature(scene, context);

  return {
    recommendation_id: `mock-rec-${scene}-${createStableRecommendationHash(signature)}-${index}`,
    text: entry.text,
    intent: {
      name: entry.intentName,
      arguments: entry.intentArgs ?? {},
    },
  };
}

/**
 * Builds the local demo-only presentation metadata for a mock recommendation.
 *
 * The coordinator uses this to render a short copy block and priority badge
 * without extending the formal recommendation protocol payload.
 */
export function buildMockRecommendationPresentation(input: {
  scene: RecommendationScene;
  context: AgentRecommendationGetParams["context"];
  index: number;
}): MockRecommendationPresentation {
  const context = input.context as ExtendedRecommendationContext;
  const entry = MOCK_RECOMMENDATION_LIBRARY[input.scene]?.[input.index] ?? MOCK_RECOMMENDATION_LIBRARY.idle[0];
  const summary = resolveRecommendationSummary({
    entry,
    scene: input.scene,
    context,
  });
  const reason = resolveRecommendationReason({
    entry,
    scene: input.scene,
    context,
  });
  const copy = normalizeRecommendationCopy([summary, reason].filter((piece) => piece.trim() !== "").join(" "));
  const priority = entry.priority;

  return {
    label: entry.label,
    copy,
    summary,
    reason,
    priority,
    priorityLabel: resolveRecommendationPriorityLabel(priority),
  };
}

/**
 * Builds a mock recommendation response for the shell-ball demo path.
 *
 * The local service intentionally stays deterministic and self-contained so the
 * desktop shell-ball can demonstrate proactive recommendation UX without
 * depending on the backend recommendation stack.
 */
export async function getMockRecommendations(
  params: AgentRecommendationGetParams,
): Promise<AgentRecommendationGetResult> {
  const scene = params.scene ?? "idle";
  const sceneRecommendations = MOCK_RECOMMENDATION_LIBRARY[scene] ?? MOCK_RECOMMENDATION_LIBRARY.idle;
  const context = params.context as ExtendedRecommendationContext;

  return {
    cooldown_hit: false,
    items: sceneRecommendations.map((entry, index) => buildMockRecommendationItem(entry, index, scene, context)),
  };
}
