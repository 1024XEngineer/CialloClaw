import type { ApprovalPendingNotification, DeliveryReadyNotification, SettingsSnapshot } from "@cialloclaw/protocol";
import { loadSettings } from "./settingsService";

type VoiceNotificationKind =
  | "startup_greeting"
  | "idle_click_greeting"
  | "selection_detected"
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
const IDLE_CLICK_GREETING_TEXT = "CialloClaw";
const SELECTION_DETECTED_TEXT = "检测到选中文本";
const CLIPBOARD_DETECTED_TEXT = "检测到剪贴板内容";
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

  if (normalizedVoiceType === "default_female") {
    return [
      language.toLowerCase(),
      "xiaoyi",
      "xiaoxiao",
      "tingting",
      "yunxia",
      "girl",
      "young",
      "female",
      "woman",
    ];
  }

  if (normalizedVoiceType === "default_male") {
    return [
      language.toLowerCase(),
      "male",
      "man",
      "boy",
      "yunxi",
      "yunyang",
      "daichi",
      "keita",
    ];
  }

  return normalizedVoiceType.length > 0 ? [normalizedVoiceType] : [language.toLowerCase()];
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
 * Builds the idle-click acknowledgement spoken from the resting floating ball.
 *
 * @returns The short idle greeting used for single-click acknowledgement.
 */
export function resolveShellBallIdleClickGreetingText() {
  return IDLE_CLICK_GREETING_TEXT;
}

function resolveVoiceNotificationText(kind: VoiceNotificationKind, payload?: ApprovalPendingNotification | DeliveryReadyNotification) {
  if (kind === "startup_greeting") {
    return resolveShellBallStartupGreetingText();
  }

  if (kind === "idle_click_greeting") {
    return resolveShellBallIdleClickGreetingText();
  }

  if (kind === "selection_detected") {
    return SELECTION_DETECTED_TEXT;
  }

  if (kind === "clipboard_detected") {
    return CLIPBOARD_DETECTED_TEXT;
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
 * Plays a short idle acknowledgement when the resting shell-ball is clicked.
 *
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakShellBallIdleGreeting() {
  return speakVoiceNotification({ kind: "idle_click_greeting" });
}

/**
 * Plays a short reminder when the shell-ball detects a fresh text selection.
 *
 * @returns Whether the local desktop runtime attempted speech playback.
 */
export function speakShellBallSelectionDetectedNotification() {
  return speakVoiceNotification({ kind: "selection_detected" });
}

/**
 * Plays a short reminder when the shell-ball receives a fresh clipboard prompt.
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
