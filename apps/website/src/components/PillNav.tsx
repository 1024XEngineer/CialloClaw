import { Github, Languages, Menu } from "lucide-react";
import { useState } from "react";
import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { useWebsiteTheme } from "@/lib/theme.tsx";
import { SearchBar } from "@/components/SearchBar";
import { SearchOverlay } from "@/components/SearchOverlay";
import { ThemeSwitch } from "@/components/ThemeSwitch";
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

type NavItem = {
  label: string;
  href: string;
};

type PillNavProps = {
  logoAlt: string;
  items: NavItem[];
  activeHref: string;
  className?: string;
  isHome?: boolean;
};

export function PillNav({ activeHref, className, isHome = false }: PillNavProps) {
  const [open, setOpen] = useState(false);
  const [searchOpen, setSearchOpen] = useState(false);
  const { isDark, toggleTheme } = useWebsiteTheme();
  const { locale, setLocale, t } = useI18n();

  return (
    <header
      className={cn(
        "fixed inset-x-0 top-0 z-50",
        !isHome && "border-b border-[color:var(--cc-line)] backdrop-blur-xl",
        className,
      )}
    >
      <div className="mx-auto flex min-h-[72px] w-full max-w-[1600px] items-center justify-between gap-4 px-5 py-4 sm:px-8 lg:px-12">
        <div className="flex min-w-0 flex-1 items-center gap-4 lg:gap-8">
          <Link to="/" className="flex shrink-0 items-center gap-3">
            <img
              src="/assets/icons/logo.png"
              alt="CialloClaw logo"
              className="h-10 w-10 object-cover"
            />
            <span
              className="text-[30px] font-medium tracking-[-0.01em] text-[color:var(--cc-ink)] leading-none"
              style={{ fontFamily: "'Montserrat', sans-serif" }}
            >
              CialloClaw
            </span>
          </Link>

          <div className="hidden max-w-[360px] flex-1 lg:block">
            <SearchBar placeholder={t("nav.search.placeholder")} onFocus={() => setSearchOpen(true)} className="w-full [&_.group]:max-w-none" />
          </div>
        </div>

        <nav className="hidden shrink-0 items-center gap-5 xl:flex">
          <Link
            to="/docs/what-is"
            className={cn(
              "text-sm font-bold transition-colors",
              activeHref.startsWith("/docs")
                ? "text-[color:var(--cc-ink)]"
                : "text-[color:var(--cc-ink-muted)] hover:text-[color:var(--cc-ink)]",
            )}
          >
            {t("nav.docs")}
          </Link>

          <DropdownMenu>
            <DropdownMenuTrigger className={navIconTriggerClassName()}>
              <Languages className="h-4 w-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent data-role="select" className="min-w-[140px]">
              <DropdownMenuCheckboxItem className="font-bold" checked={locale === "zh"} onCheckedChange={() => setLocale("zh")}>
                {t("nav.lang.zh")}
              </DropdownMenuCheckboxItem>
              <DropdownMenuCheckboxItem className="font-bold" checked={locale === "en"} onCheckedChange={() => setLocale("en")}>
                {t("nav.lang.en")}
              </DropdownMenuCheckboxItem>
            </DropdownMenuContent>
          </DropdownMenu>

          <ThemeSwitch checked={isDark} onChange={toggleTheme} />

          <a
            href="https://github.com/1024XEngineer/CialloClaw"
            target="_blank"
            rel="noreferrer"
            className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-[color:var(--cc-line)] bg-[color:var(--cc-surface)] text-[color:var(--cc-ink-soft)] backdrop-blur-md transition hover:bg-[color:var(--cc-surface-strong)] hover:text-[color:var(--cc-ink)]"
          >
            <Github className="h-4 w-4" />
          </a>
        </nav>

        <button
          type="button"
          className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-[color:var(--cc-line)] bg-[color:var(--cc-surface)] text-[color:var(--cc-ink)] xl:hidden"
          onClick={() => setOpen((value) => !value)}
        >
          <Menu className="h-5 w-5" />
        </button>
      </div>

      {open ? (
        <div className="absolute inset-x-0 top-full border-t border-[color:var(--cc-line)] bg-[color:var(--cc-bg)] px-5 py-4 backdrop-blur-xl xl:hidden">
          <div className="mx-auto flex max-w-[1600px] flex-col gap-2">
            <div className="flex h-11 items-center gap-3 rounded-2xl border border-[color:var(--cc-line)] bg-[color:var(--cc-surface)] px-4">
              <SearchBar placeholder={t("nav.search.placeholder")} onFocus={() => setSearchOpen(true)} className="w-full [&_.group]:max-w-none [&_.input]:shadow-none [&_.input]:bg-transparent [&_.input]:h-9 [&_.input]:pl-10 [&_.search-icon]:left-3" />
            </div>
            <Link to="/docs/what-is" className="rounded-2xl px-4 py-3 text-sm font-bold text-[color:var(--cc-ink-soft)] hover:bg-[color:var(--cc-surface)] hover:text-[color:var(--cc-ink)]" onClick={() => setOpen(false)}>{t("nav.docs")}</Link>
            <button type="button" className="rounded-2xl px-4 py-3 text-left text-sm font-medium text-[color:var(--cc-ink-soft)] hover:bg-[color:var(--cc-surface)] hover:text-[color:var(--cc-ink)]" onClick={() => setLocale(locale === "zh" ? "en" : "zh")}>{locale === "zh" ? t("nav.lang.zh") : t("nav.lang.en")}</button>
            <button type="button" className="rounded-2xl px-4 py-3 text-left text-sm font-medium text-[color:var(--cc-ink-soft)] hover:bg-[color:var(--cc-surface)] hover:text-[color:var(--cc-ink)]" onClick={toggleTheme}>{isDark ? t("nav.light") : t("nav.dark")}</button>
            <a href="https://github.com/1024XEngineer/CialloClaw" target="_blank" rel="noreferrer" className="rounded-2xl px-4 py-3 text-sm font-medium text-[color:var(--cc-ink-soft)] hover:bg-[color:var(--cc-surface)] hover:text-[color:var(--cc-ink)]">
              GitHub
            </a>
          </div>
        </div>
      ) : null}

      <SearchOverlay open={searchOpen} onClose={() => setSearchOpen(false)} />
    </header>
  );
}

function navIconTriggerClassName() {
  return "inline-flex h-10 w-10 items-center justify-center rounded-full border border-[color:var(--cc-line)] bg-[color:var(--cc-surface)] text-[color:var(--cc-ink-soft)] backdrop-blur-md outline-none transition hover:bg-[color:var(--cc-surface-strong)] hover:text-[color:var(--cc-ink)] focus:outline-none focus-visible:outline-none focus-visible:ring-0 active:outline-none";
}
