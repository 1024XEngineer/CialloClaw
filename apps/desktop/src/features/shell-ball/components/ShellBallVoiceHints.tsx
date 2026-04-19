import { ArrowDown, ArrowUp, Check, Lock, X } from "lucide-react";
import { cn } from "../../../utils/cn";
import type { ShellBallVoicePreview } from "../shellBall.interaction";
import type { ShellBallVoiceHintMode } from "../shellBall.types";

type ShellBallVoiceHintsProps = {
  hintMode: ShellBallVoiceHintMode;
  voicePreview: ShellBallVoicePreview;
};

export function ShellBallVoiceHints({
  hintMode,
  voicePreview,
}: ShellBallVoiceHintsProps) {
  if (hintMode === "hidden") {
    return null;
  }

  return (
    <>
      {hintMode === "lock" ? (
        <div
          className={cn(
            "shell-ball-mascot__voice-hint",
            "shell-ball-mascot__voice-hint--lock",
            voicePreview === "lock" && "is-active",
          )}
        >
          <ArrowUp className="shell-ball-mascot__voice-arrow" />
          <Lock className="shell-ball-mascot__voice-icon" />
        </div>
      ) : null}

      {hintMode === "cancel" ? (
        <div
          className={cn(
            "shell-ball-mascot__voice-hint",
            "shell-ball-mascot__voice-hint--cancel",
            voicePreview === "cancel" && "is-active",
          )}
        >
          <ArrowDown className="shell-ball-mascot__voice-arrow" />
          <X className="shell-ball-mascot__voice-icon" />
        </div>
      ) : null}

      {hintMode === "finish" ? (
        <div
          className={cn(
            "shell-ball-mascot__voice-hint",
            "shell-ball-mascot__voice-hint--finish",
            voicePreview === "cancel" && "is-active",
          )}
        >
          <ArrowDown className="shell-ball-mascot__voice-arrow" />
          <Check className="shell-ball-mascot__voice-icon" />
        </div>
      ) : null}
    </>
  );
}
