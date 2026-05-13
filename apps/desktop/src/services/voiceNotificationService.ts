import type { ApprovalPendingNotification, DeliveryReadyNotification, SettingsSnapshot } from "@cialloclaw/protocol";
import { loadSettings } from "./settingsService";
import { DEFAULT_VOICE_NOTIFICATION_VOICE_TYPE } from "./voiceNotificationConfig";

type VoiceNotificationKind =
  | "startup_greeting"
  | "selection_detected"
  | "selection_click_greeting"
  | "clipboard_detected"
  | "approval_pending"
  | "delivery_ready";

type VoiceNotificationSettings = SettingsSnapshot["settings"]["general"];

type SpeechVoiceLike = {
  lang: string;
  name: string;
};

type SpeechSynthesisLike = {
  addEventListener?: (type: "voiceschanged", listener: () => void) => void;
  cancel: () => void;
  getVoices: () => SpeechVoiceLike[];
  removeEventListener?: (type: "voiceschanged", listener: () => void) => void;
  speak: (utterance: SpeechSynthesisUtterance) => void;
};

const STARTUP_GREETING_TEXT = "CialloClaw 已启动";
const SELECTION_CLICK_GREETING_TEXT = "Ciallo";
const SELECTION_DETECTED_TEXT_1 = "发现一段高亮文字啦";
const SELECTION_DETECTED_TEXT_2 = "我接住新选中的内容啦";
const SELECTION_DETECTED_TEXT_3 = "新的选中内容到啦";
const SELECTION_DETECTED_TEXT_4 = "这段选中文字交给我吧";
const SELECTION_DETECTED_TEXT_5 = "我看到你圈出的这段话啦";
const CLIPBOARD_DETECTED_TEXT_1 = "剪贴板刚刚更新啦";
const CLIPBOARD_DETECTED_TEXT_2 = "我接住新复制的内容啦";
const CLIPBOARD_DETECTED_TEXT_3 = "新的剪贴板内容到啦";
const CLIPBOARD_DETECTED_TEXT_4 = "这段复制内容交给我吧";
const CLIPBOARD_DETECTED_TEXT_5 = "我看到你刚复制的小纸条啦";
const APPROVAL_PENDING_TEXT = "有一个操作需要你确认";
const DELIVERY_READY_TEXT = "任务结果已准备好";
const VOICE_LIST_READY_TIMEOUT_MS = 600;
let latestVoiceNotificationRequestId = 0;

function normalizeVoiceType(voiceType: string) {
  return voiceType.trim().toLowerCase();
}

function normalizeVoiceLabel(value: string) {
  return value.trim().toLowerCase();
}

function resolveRandomVoiceNotificationText(options: readonly string[], randomValue = Math.random()) {
  if (options.length === 0) {
    return "";
  }

  const normalizedRandomValue = Number.isFinite(randomValue) ? randomValue : 0;
  const boundedRandomValue = Math.min(Math.max(normalizedRandomValue, 0), 0.999_999_999_999);
  const resolvedIndex = Math.min(options.length - 1, Math.floor(boundedRandomValue * options.length));

  return options[resolvedIndex] ?? options[0] ?? "";
}

function getVoiceNotificationSettings(): VoiceNotificationSettings {
  return loadSettings().settings.general;
}

function getVoiceNotificationHost() {
  if (typeof window === "undefined" || typeof window.speechSynthesis === "undefined" || typeof SpeechSynthesisUtterance === "undefined") {
    return null;
  }

  return {
    synthesizer: window.speechSynthesis as SpeechSynthesisLike,
    utteranceConstructor: SpeechSynthesisUtterance,
  };
}

function getVoiceTypePreferences(voiceType: string, language: string) {
  const normalizedVoiceType = normalizeVoiceType(voiceType);
  const languagePreference = language.toLowerCase();

  if (normalizedVoiceType === "default_female") {
    return [
      languagePreference,
      "xiaoxiao",
      "xiaoyi",
      "tongtong",
      "xiaohan",
      "child",
      "kid",
      "tingting",
      "yunxia",
      "girl",
      "young",
      "cute",
      "female",
      "woman",
    ];
  }

  if (normalizedVoiceType === "soft_girl") {
    return [
      languagePreference,
      "xiaoxiao",
      "xiaohan",
      "tongtong",
      "soft",
      "gentle",
      "cute",
      "child",
      "girl",
      "female",
    ];
  }

  if (normalizedVoiceType === "bright_girl") {
    return [
      languagePreference,
      "xiaoyi",
      "tingting",
      "xiaoxuan",
      "bright",
      "lively",
      "young",
      "girl",
      "female",
    ];
  }

  if (normalizedVoiceType === "default_male") {
    return [
      languagePreference,
      "male",
      "man",
      "boy",
      "yunxi",
      "yunyang",
      "daichi",
      "keita",
    ];
  }

  if (normalizedVoiceType.length === 0) {
    return getVoiceTypePreferences(DEFAULT_VOICE_NOTIFICATION_VOICE_TYPE, language);
  }

  return [normalizedVoiceType];
}

/**
 * Picks the closest available system voice for the saved desktop preference.
 *
 * @param input Available voices plus the preferred voice type and language.
 * @returns The best matching speech synthesis voice or `null` when none exist.
 */
export function resolveVoiceNotificationVoice(input: {
  language: string;
  voiceType: string;
  voices: SpeechVoiceLike[];
}) {
  if (input.voices.length === 0) {
    return null;
  }

  const language = input.language.trim().toLowerCase();
  const preferredLanguagePrefix = language.split("-")[0] ?? language;
  const normalizedVoiceType = normalizeVoiceType(input.voiceType);
  const preferences = getVoiceTypePreferences(input.voiceType, input.language);

  let bestVoice: SpeechVoiceLike | null = null;
  let bestScore = Number.NEGATIVE_INFINITY;

  input.voices.forEach((voice) => {
    const normalizedName = normalizeVoiceLabel(voice.name);
    const normalizedLanguage = normalizeVoiceLabel(voice.lang);
    const normalizedLanguagePrefix = normalizedLanguage.split("-")[0] ?? normalizedLanguage;
    let score = 0;

    if (normalizedVoiceType.length > 0 && normalizedName === normalizedVoiceType) {
      score += 100;
    }

    preferences.forEach((preference, index) => {
      if (normalizedName.includes(preference) || normalizedLanguage.includes(preference)) {
        score += 30 - index;
      }
    });

    if (normalizedLanguage === language) {
      score += 20;
    } else if (normalizedLanguagePrefix === preferredLanguagePrefix) {
      score += 10;
    }

    if (score > bestScore) {
      bestVoice = voice;
      bestScore = score;
    }
  });

  return bestVoice ?? input.voices[0] ?? null;
}

/**
 * Decides the spoken copy for a formal delivery notification.
 *
 * @param payload The stable `delivery.ready` notification payload.
 * @returns The short spoken line, or `null` when the delivery should stay silent.
 */
export function resolveDeliveryReadyVoiceNotificationText(payload: DeliveryReadyNotification) {
  return payload.delivery_result.type === "bubble" ? null : DELIVERY_READY_TEXT;
}

/**
 * Builds the startup greeting text for the floating ball window.
 *
 * @returns The short greeting that should be spoken after startup.
 */
export function resolveShellBallStartupGreetingText() {
  return STARTUP_GREETING_TEXT;
}

/**
 * Builds a playful selection-detected reminder for the floating ball so
 * repeated demos feel less robotic without expanding the notification scope.
 *
 * @param randomValue Optional deterministic random input for contract tests.
 * @returns One of five local selection-detected reminder lines.
 */
export function resolveShellBallSelectionDetectedNotificationText(randomValue = Math.random()) {
  return resolveRandomVoiceNotificationText([
    SELECTION_DETECTED_TEXT_1,
    SELECTION_DETECTED_TEXT_2,
    SELECTION_DETECTED_TEXT_3,
    SELECTION_DETECTED_TEXT_4,
    SELECTION_DETECTED_TEXT_5,
  ], randomValue);
}

/**
 * Builds the short `Ciallo` acknowledgement spoken when the user accepts a
 * fresh selected-text reminder from the floating ball.
 *
 * @returns The short acknowledgement used for the selected-text prompt click.
 */
export function resolveShellBallSelectionClickGreetingText() {
  return SELECTION_CLICK_GREETING_TEXT;
}

/**
 * Builds a playful clipboard-detected reminder for the floating ball so local
 * clipboard prompts feel lively during repeated desktop demos.
 *
 * @param randomValue Optional deterministic random input for contract tests.
 * @returns One of five local clipboard-detected reminder lines.
 */
export function resolveShellBallClipboardDetectedNotificationText(randomValue = Math.random()) {
  return resolveRandomVoiceNotificationText([
    CLIPBOARD_DETECTED_TEXT_1,
    CLIPBOARD_DETECTED_TEXT_2,
    CLIPBOARD_DETECTED_TEXT_3,
    CLIPBOARD_DETECTED_TEXT_4,
    CLIPBOARD_DETECTED_TEXT_5,
  ], randomValue);
}

function resolveVoiceNotificationText(kind: VoiceNotificationKind, payload?: ApprovalPendingNotification | DeliveryReadyNotification) {
  if (kind === "startup_greeting") {
    return resolveShellBallStartupGreetingText();
  }

  if (kind === "selection_detected") {
    return resolveShellBallSelectionDetectedNotificationText();
  }

  if (kind === "selection_click_greeting") {
    return resolveShellBallSelectionClickGreetingText();
  }

  if (kind === "clipboard_detected") {
    return resolveShellBallClipboardDetectedNotificationText();
  }

  if (kind === "approval_pending") {
    return APPROVAL_PENDING_TEXT;
  }

  if (!payload || !("delivery_result" in payload)) {
    return null;
  }

  return resolveDeliveryReadyVoiceNotificationText(payload);
}

function resolveVoiceNotificationLanguage() {
  return "zh-CN";
}

function waitForVoiceNotificationVoices(synthesizer: SpeechSynthesisLike) {
  const availableVoices = synthesizer.getVoices();
  if (availableVoices.length > 0 || synthesizer.addEventListener === undefined || synthesizer.removeEventListener === undefined) {
    return Promise.resolve(availableVoices);
  }

  return new Promise<SpeechVoiceLike[]>((resolve) => {
    let settled = false;

    const finalize = () => {
      if (settled) {
        return;
      }

      settled = true;
      window.clearTimeout(timeoutId);
      synthesizer.removeEventListener?.("voiceschanged", handleVoicesChanged);
      resolve(synthesizer.getVoices());
    };

    const handleVoicesChanged = () => {
      finalize();
    };

    // Chromium can populate speech voices after the first startup paint, so a
    // short bounded wait keeps the saved desktop voice preference effective.
    const timeoutId = window.setTimeout(() => {
      finalize();
    }, VOICE_LIST_READY_TIMEOUT_MS);

    synthesizer.addEventListener("voiceschanged", handleVoicesChanged);
  });
}

async function speakVoiceNotification(input: {
  kind: VoiceNotificationKind;
  payload?: ApprovalPendingNotification | DeliveryReadyNotification;
}) {
  const requestId = ++latestVoiceNotificationRequestId;
  const host = getVoiceNotificationHost();
  if (host === null) {
    return false;
  }

  const initialSettings = getVoiceNotificationSettings();
  if (!initialSettings.voice_notification_enabled) {
    return false;
  }

  const text = resolveVoiceNotificationText(input.kind, input.payload);
  if (text === null) {
    return false;
  }

  const language = resolveVoiceNotificationLanguage();
  const utterance = new host.utteranceConstructor(text);
  const availableVoices = await waitForVoiceNotificationVoices(host.synthesizer);

  // Voice discovery can resolve out of order during startup, so only the most
  // recent local notification is allowed to claim the shared synthesizer.
  if (requestId !== latestVoiceNotificationRequestId) {
    return false;
  }

  // The control panel can change local voice settings while the browser is
  // still resolving system voices, so re-read the latest snapshot right before
  // claiming the shared synthesizer.
  const latestSettings = getVoiceNotificationSettings();
  if (!latestSettings.voice_notification_enabled) {
    return false;
  }

  const resolvedVoice = resolveVoiceNotificationVoice({
    language,
    voiceType: latestSettings.voice_type,
    voices: availableVoices,
  });

  utterance.lang = language;
  if (resolvedVoice !== null) {
    utterance.voice = resolvedVoice as SpeechSynthesisVoice;
  }

  host.synthesizer.cancel();
  host.synthesizer.speak(utterance);
  return true;
}

/**
 * Plays the startup greeting once the floating ball becomes available.
 *
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakShellBallStartupGreeting() {
  return speakVoiceNotification({ kind: "startup_greeting" });
}

/**
 * Plays a short local reminder when the floating ball captures new selected
 * text and exposes the click-to-start affordance.
 *
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakShellBallSelectionDetectedNotification() {
  return speakVoiceNotification({ kind: "selection_detected" });
}

/**
 * Plays a short `Ciallo` acknowledgement when the user accepts a selected-text
 * reminder from the floating ball.
 *
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakShellBallSelectionClickGreeting() {
  return speakVoiceNotification({ kind: "selection_click_greeting" });
}

/**
 * Plays a short local reminder when the floating ball captures new clipboard
 * text and exposes the click-to-submit affordance.
 *
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakShellBallClipboardDetectedNotification() {
  return speakVoiceNotification({ kind: "clipboard_detected" });
}

/**
 * Plays a short approval reminder for the formal `approval.pending` notification.
 *
 * @param payload The stable task approval notification payload.
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakApprovalPendingNotification(payload: ApprovalPendingNotification) {
  return speakVoiceNotification({
    kind: "approval_pending",
    payload,
  });
}

/**
 * Plays a short delivery reminder for non-bubble formal delivery results.
 *
 * @param payload The stable task delivery notification payload.
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakDeliveryReadyNotification(payload: DeliveryReadyNotification) {
  return speakVoiceNotification({
    kind: "delivery_ready",
    payload,
  });
}
