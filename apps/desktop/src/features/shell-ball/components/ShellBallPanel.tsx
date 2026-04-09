import { StatusBadge } from "@cialloclaw/ui";
import { AlertTriangle, Mic } from "lucide-react";
import { cn } from "../../../utils/cn";
import { getShellBallPanelSections } from "../shellBall.demo";
import type { ShellBallAccentTone, ShellBallDemoViewModel, ShellBallPanelSection } from "../shellBall.types";

type ShellBallPanelProps = {
  viewModel: ShellBallDemoViewModel;
  accentTone: ShellBallAccentTone;
};

function hasSection(sections: readonly ShellBallPanelSection[], section: ShellBallPanelSection) {
  return sections.includes(section);
}

export function ShellBallPanel({ viewModel, accentTone }: ShellBallPanelProps) {
  if (viewModel.panelMode === "hidden") {
    return null;
  }

  const sections = getShellBallPanelSections(viewModel);

  return (
    <section className={cn("shell-ball-panel", `shell-ball-panel--${viewModel.panelMode}`)} data-tone={accentTone}>
      {hasSection(sections, "badge") ? <StatusBadge tone={viewModel.badgeTone}>{viewModel.badgeLabel}</StatusBadge> : null}
      {hasSection(sections, "title") ? <h2 className="shell-ball-panel__title">{viewModel.title}</h2> : null}
      {hasSection(sections, "subtitle") ? <p className="shell-ball-panel__subtitle">{viewModel.subtitle}</p> : null}
      {hasSection(sections, "helperText") ? <p className="shell-ball-panel__helper">{viewModel.helperText}</p> : null}

      {hasSection(sections, "risk") && viewModel.showRiskBlock ? (
        <div className="shell-ball-panel__block shell-ball-panel__block--risk">
          <div className="shell-ball-panel__block-header">
            <AlertTriangle className="shell-ball-panel__block-icon" />
            <p className="shell-ball-panel__block-title">{viewModel.riskTitle}</p>
          </div>
          <p className="shell-ball-panel__block-copy">{viewModel.riskText}</p>
        </div>
      ) : null}

      {hasSection(sections, "voiceHint") && viewModel.showVoiceHint ? (
        <div className="shell-ball-panel__block shell-ball-panel__block--voice">
          <div className="shell-ball-panel__voice-row">
            <Mic className="shell-ball-panel__block-icon" />
            <p className="shell-ball-panel__block-copy">{viewModel.voiceHintText}</p>
          </div>
        </div>
      ) : null}
    </section>
  );
}
