export const MIRROR_DIRECTION_KEYS = ["dailyStage", "profile", "memory", "history"] as const;

export type MirrorDirectionKey = (typeof MIRROR_DIRECTION_KEYS)[number];
export type FloatingMirrorDirectionKey = Exclude<MirrorDirectionKey, "memory">;
export type MirrorCardAccent = "sky" | "warm" | "sage" | "rose";

export type MirrorDirectionMeta = {
  title: string;
  eyebrow: string;
  accent: MirrorCardAccent;
  hint: string;
};

export const FLOATING_MIRROR_DIRECTION_KEYS = MIRROR_DIRECTION_KEYS.filter(
  (key): key is FloatingMirrorDirectionKey => key !== "memory",
);

export const DEFAULT_MIRROR_DIRECTION_STACK: MirrorDirectionKey[] = [...MIRROR_DIRECTION_KEYS];

export const MIRROR_DIRECTION_META: Record<MirrorDirectionKey, MirrorDirectionMeta> = {
  dailyStage: {
    title: "日报与阶段总结",
    eyebrow: "日报与阶段",
    accent: "warm",
    hint: "拖动载片 · 点按查看",
  },
  profile: {
    title: "用户画像",
    eyebrow: "用户画像",
    accent: "sage",
    hint: "拖动载片 · 点按查看",
  },
  memory: {
    title: "近期被调用记忆",
    eyebrow: "记忆引用",
    accent: "sky",
    hint: "固定母片 · 点按查看",
  },
  history: {
    title: "历史概要",
    eyebrow: "历史概要",
    accent: "rose",
    hint: "拖动载片 · 点按查看",
  },
};

export const MIRROR_ORBITAL_TARGETS: Record<FloatingMirrorDirectionKey, { x: number; y: number }> = {
  dailyStage: { x: 0.42, y: 0.76 },
  profile: { x: 0.72, y: 0.46 },
  history: { x: 0.74, y: 0.12 },
};

export function getMirrorDirectionMeta(key: MirrorDirectionKey) {
  return MIRROR_DIRECTION_META[key];
}
