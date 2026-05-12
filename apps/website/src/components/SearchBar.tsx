import styled from "styled-components";

type SearchBarProps = {
  placeholder?: string;
  value?: string;
  onChange?: (value: string) => void;
  onFocus?: () => void;
  autoFocus?: boolean;
  className?: string;
};

export function SearchBar({ placeholder = "Search...", value, onChange, onFocus, autoFocus, className }: SearchBarProps) {
  return (
    <SearchShell className={className}>
      <div className="group">
        <svg viewBox="0 0 24 24" aria-hidden="true" className="search-icon">
          <g>
            <path d="M21.53 20.47l-3.66-3.66C19.195 15.24 20 13.214 20 11c0-4.97-4.03-9-9-9s-9 4.03-9 9 4.03 9 9 9c2.215 0 4.24-.804 5.808-2.13l3.66 3.66c.147.146.34.22.53.22s.385-.073.53-.22c.295-.293.295-.767.002-1.06zM3.5 11c0-4.135 3.365-7.5 7.5-7.5s7.5 3.365 7.5 7.5-3.365 7.5-7.5 7.5-7.5-3.365-7.5-7.5z" />
          </g>
        </svg>
        <input
          id="query"
          className="input"
          type="search"
          placeholder={placeholder}
          name="searchbar"
          value={value}
          onChange={(event) => onChange?.(event.target.value)}
          onFocus={onFocus}
          autoFocus={autoFocus}
        />
      </div>
    </SearchShell>
  );
}

const SearchShell = styled.div`
  .group {
    display: flex;
    line-height: 28px;
    align-items: center;
    position: relative;
    width: 100%;
    max-width: 240px;
  }

  .input {
    font-family: "Montserrat", var(--font-sans, sans-serif);
    width: 100%;
    height: 45px;
    padding-left: 2.65rem;
    box-shadow: 0 0 0 1.5px var(--cc-line), 0 0 25px -17px rgba(0, 0, 0, 0.32);
    border: 0;
    border-radius: 12px;
    background-color: var(--cc-surface);
    outline: none;
    color: var(--cc-ink);
    transition: all 0.25s cubic-bezier(0.19, 1, 0.22, 1);
    cursor: text;
    z-index: 0;
    backdrop-filter: blur(18px);
  }

  .input::placeholder {
    color: var(--cc-ink-muted);
  }

  .input:hover {
    box-shadow: 0 0 0 2px var(--cc-line-strong), 0px 0px 25px -15px rgba(0, 0, 0, 0.24);
  }

  .input:active {
    transform: scale(0.98);
  }

  .input:focus {
    box-shadow: 0 0 0 2px rgba(170, 191, 255, 0.38), 0 20px 40px -30px rgba(103, 147, 255, 0.35);
  }

  .search-icon {
    position: absolute;
    left: 1rem;
    fill: var(--cc-ink-muted);
    width: 1rem;
    height: 1rem;
    pointer-events: none;
    z-index: 1;
  }
`;
