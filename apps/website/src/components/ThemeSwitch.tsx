import styled from "styled-components";

type ThemeSwitchProps = {
  checked: boolean;
  onChange: (checked: boolean) => void;
};

export function ThemeSwitch({ checked, onChange }: ThemeSwitchProps) {
  return (
    <SwitchShell>
      <label className="ui-switch" aria-label="Toggle theme">
        <input type="checkbox" checked={checked} onChange={(event) => onChange(event.target.checked)} />
        <div className="slider">
          <div className="circle" />
        </div>
      </label>
    </SwitchShell>
  );
}

const SwitchShell = styled.div`
  .ui-switch {
    --switch-bg: rgb(135, 150, 165);
    --switch-width: 48px;
    --switch-height: 20px;
    --circle-diameter: 32px;
    --circle-bg-off: rgb(130, 0, 0);
    --circle-bg-on: rgb(0, 56, 146);
    --circle-inset: calc((var(--circle-diameter) - var(--switch-height)) / 2);
    display: inline-flex;
  }

  .ui-switch input {
    display: none;
  }

  .slider {
    appearance: none;
    width: var(--switch-width);
    height: var(--switch-height);
    background: var(--switch-bg);
    border-radius: 999px;
    position: relative;
    cursor: pointer;
  }

  .slider .circle {
    top: calc(var(--circle-inset) * -1);
    left: 0;
    width: var(--circle-diameter);
    height: var(--circle-diameter);
    position: absolute;
    background: var(--circle-bg-off);
    border-radius: inherit;
    background-image: url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIGhlaWdodD0iMjAiIHdpZHRoPSIyMCIgdmlld0JveD0iMCAwIDIwIDIwIj4KICAgIDxwYXRoIGZpbGw9IiNmZmYiCiAgICAgICAgZD0iTTkuMzA1IDEuNjY3VjMuNzVoMS4zODlWMS42NjdoLTEuMzl6bS00LjcwNyAxLjk1bC0uOTgyLjk4Mkw1LjA5IDYuMDcybC45ODItLjk4Mi0xLjQ3My0xLjQ3M3ptMTAuODAyIDBMMTMuOTI3IDUuMDlsLjk4Mi45ODIgMS40NzMtMS40NzMtLjk4Mi0uOTgyek0xMCA1LjEzOWE0Ljg3MiA0Ljg3MiAwIDAwLTQuODYyIDQuODZBNC44NzIgNC44NzIgMCAwMDEwIDE0Ljg2MiA0Ljg3MiA0Ljg3MiAwIDAwMTQuODYgMTAgNC44NzIgNC44NzIgMCAwMDEwIDUuMTM5em0wIDEuMzg5QTMuNDYyIDMuNDYyIDAgMDExMy40NzEgMTBhMy40NjIgMy40NjIgMCAwMS0zLjQ3MyAzLjQ3MkEzLjQ2MiAzLjQ2MiAwIDAxNi41MjcgMTAgMy40NjIgMy40NjIgMCAwMTEwIDYuNTI4ek0xLjY2NSA5LjMwNXYxLjM5aDIuMDgzdi0xLjM5SDEuNjY2em0xNC41ODMgMHYxLjM5aDIuMDg0di0xLjM5aC0yLjA4NHpNNS4wOSAxMy45MjhMMy42MTYgMTUuNGwuOTgyLjk4MiAxLjQ3My0xLjQ3My0uOTgyLS45ODJ6bTkuODIgMGwtLjk4Mi45ODIgMS40NzMgMS40NzMuOTgyLS45ODItMS40NzMtMS40NzN6TTkuMzA1IDE2LjI1djIuMDgzaDEuMzg5VjE2LjI1aC0xLjM5eiIgLz4KPC9zdmc+");
    background-repeat: no-repeat;
    background-position: center center;
    transition: left 150ms cubic-bezier(0.4, 0, 0.2, 1), transform 150ms cubic-bezier(0.4, 0, 0.2, 1);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0px 2px 1px -1px rgba(0, 0, 0, 0.2), 0px 1px 1px 0px rgba(0, 0, 0, 0.14), 0px 1px 3px 0px rgba(0, 0, 0, 0.12);
  }

  .slider .circle::before {
    content: "";
    position: absolute;
    width: 100%;
    height: 100%;
    background: rgba(255, 255, 255, 0.75);
    border-radius: inherit;
    transition: all 500ms;
    opacity: 0;
  }

  .ui-switch input:checked + .slider .circle {
    left: calc(100% - var(--circle-diameter));
    background: var(--circle-bg-on);
    background-image: url("data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIGhlaWdodD0iMjAiIHdpZHRoPSIyMCIgdmlld0JveD0iMCAwIDIwIDIwIj4KICAgIDxwYXRoIGZpbGw9IiNmZmYiCiAgICAgICAgZD0iTTQuMiAyLjVsLS43IDEuOC0xLjguNyAxLjguNy43IDEuOC42LTEuOEw2LjcgNWwtMS45LS43LS42LTEuOHptMTUgOC4zYTYuNyA2LjcgMCAxMS02LjYtNi42IDUuOCA1LjggMCAwMDYuNiA2LjZ6IiAvPgo8L3N2Zz4=");
  }

  .ui-switch input:active + .slider .circle::before {
    transition: 0s;
    opacity: 1;
    width: 0;
    height: 0;
  }
`;
