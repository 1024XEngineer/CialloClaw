import type { ApprovalPendingNotification, DeliveryReadyNotification, SettingsSnapshot } from "@cialloclaw/protocol";
import { loadSettings } from "./settingsService";

type VoiceNotificationKind = "startup_greeting" | "approval_pending" | "delivery_ready";

type VoiceNotificationSettings = SettingsSnapshot["settings"]["general"];

type SpeechVoiceLike = {
  lang: string;
  name: string;
};

type SpeechSynthesisLike = {
  cancel: () => void;
  getVoices: () => SpeechVoiceLike[];
  speak: (utterance: SpeechSynthesisUtterance) => void;
};

const STARTUP_GREETING_TEXT = "Ciallo!";
const APPROVAL_PENDING_TEXT = "有一个操作需要你确认";
const DELIVERY_READY_TEXT = "任务结果已准备好";

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
      "ja-jp",
      "female",
      "woman",
      "girl",
      "kyoko",
      "sakura",
      "nanami",
      "sayaka",
      "xiaoxiao",
      "tingting",
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

function resolveVoiceNotificationText(kind: VoiceNotificationKind, payload?: ApprovalPendingNotification | DeliveryReadyNotification) {
  if (kind === "startup_greeting") {
    return resolveShellBallStartupGreetingText();
  }

  if (kind === "approval_pending") {
    return APPROVAL_PENDING_TEXT;
  }

  if (!payload || !("delivery_result" in payload)) {
    return null;
  }

  return resolveDeliveryReadyVoiceNotificationText(payload);
}

function resolveVoiceNotificationLanguage(kind: VoiceNotificationKind) {
  return kind === "startup_greeting" ? "ja-JP" : "zh-CN";
}

function speakVoiceNotification(input: {
  kind: VoiceNotificationKind;
  payload?: ApprovalPendingNotification | DeliveryReadyNotification;
}) {
  const host = getVoiceNotificationHost();
  if (host === null) {
    return false;
  }

  const settings = getVoiceNotificationSettings();
  if (!settings.voice_notification_enabled) {
    return false;
  }

  const text = resolveVoiceNotificationText(input.kind, input.payload);
  if (text === null) {
    return false;
  }

  const language = resolveVoiceNotificationLanguage(input.kind);
  const utterance = new host.utteranceConstructor(text);
  const availableVoices = host.synthesizer.getVoices();
  const resolvedVoice = resolveVoiceNotificationVoice({
    language,
    voiceType: settings.voice_type,
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
