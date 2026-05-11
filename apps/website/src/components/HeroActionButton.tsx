import { useNavigate } from "react-router-dom";
import styled from "styled-components";

type HeroActionButtonProps = {
  label: string;
  href: string;
};

export function HeroActionButton({ label, href }: HeroActionButtonProps) {
  const navigate = useNavigate();

  const handleClick = () => {
    void navigate(href);
  };

  return (
    <ActionButtonShell>
      <button type="button" onClick={handleClick}>{label}</button>
    </ActionButtonShell>
  );
}

const ActionButtonShell = styled.div`
  button {
    --color: rgb(123, 138, 235);
    font-family: inherit;
    display: inline-block;
    width: 8em;
    height: 2.6em;
    line-height: 2.5em;
    margin: 0;
    position: relative;
    cursor: pointer;
    overflow: hidden;
    border: none;
    transition: color 0.2s ease-in-out, transform 0.2s ease-in-out, background-color 0.2s ease-in-out, box-shadow 0.2s ease-in-out;
    z-index: 1;
    font-size: 17px;
    border-radius: 0.75rem;
    font-weight: 800;
    color: var(--cc-ink);
    text-align: center;
    text-decoration: none;
    text-wrap: nowrap;
    background: color-mix(in srgb, rgb(221 235 255 / 60%) 20%, transparent);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18), 0 10px 30px rgba(37, 56, 118, 0.1);
    backdrop-filter: blur(15px);
    outline: none;
  }

  html[data-theme="dark"] & button {
    color: rgb(89 67 247);
  }

  button::before {
    content: "";
    position: absolute;
    z-index: -1;
    background: var(--color);
    height: 150px;
    width: 200px;
    border-radius: 50%;
    top: 100%;
    left: 100%;
    transition: all 0.7s;
  }

  button:hover {
    color: var(--cc-ink);
    transform: translateY(-2px);
    background: color-mix(in srgb, var(--color) 28%, transparent);
    box-shadow: 0 18px 36px rgba(65, 86, 173, 0.16);
  }

  html[data-theme="dark"] & button:hover {
    color: #ffffff;
  }

  button:hover::before {
    top: -30px;
    left: -30px;
  }

  button:active::before {
    background: #5577eb;
    transition: background 0s;
  }

  button:active {
    transform: scale(0.95);
  }

  button:focus,
  button:focus-visible {
    outline: none;
    box-shadow: 0 18px 36px rgba(65, 86, 173, 0.16);
  }
`;
