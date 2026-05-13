export const DEFAULT_VOICE_NOTIFICATION_VOICE_TYPE = "default_female";

export const VOICE_NOTIFICATION_VOICE_PRESET_OPTIONS = [
  { label: "小女孩", value: "default_female" },
  { label: "软萌", value: "soft_girl" },
  { label: "元气", value: "bright_girl" },
  { label: "少年", value: "default_male" },
] as const;

export type VoiceNotificationVoicePreset = (typeof VOICE_NOTIFICATION_VOICE_PRESET_OPTIONS)[number]["value"];

/**
 * Normalizes persisted voice settings into one of the supported preset values
 * when the control panel needs to render a radio-style choice group.
 *
 * @param voiceType Persisted `general.voice_type` value.
 * @returns The matching preset or the refined default girl preset.
 */
export function resolveVoiceNotificationVoicePreset(voiceType: string): VoiceNotificationVoicePreset {
  const normalizedVoiceType = voiceType.trim().toLowerCase();
  const matchedPreset = VOICE_NOTIFICATION_VOICE_PRESET_OPTIONS.find((option) => option.value === normalizedVoiceType);

  return matchedPreset?.value ?? DEFAULT_VOICE_NOTIFICATION_VOICE_TYPE;
}
