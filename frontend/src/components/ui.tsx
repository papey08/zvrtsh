import type { ReactNode } from "react";
import { useAuth } from "../lib/auth";
import { useConnection, type ConnectionStatus } from "../lib/events";
import { links } from "../lib/router";

export function LogoMark({ size = 36 }: { size?: number }) {
  return (
    <svg className="logo-mark" width={size} height={size} viewBox="0 0 40 40" aria-hidden="true">
      <path d="M5 7 L35 4 L35 14 L5 15.5 Z" fill="var(--paper)" />
      <path d="M5 15.5 L35 14 L35 25 L5 23.5 Z" fill="var(--paper-fold)" />
      <path d="M5 23.5 L35 25 L35 35.5 L5 33.5 Z" fill="var(--paper)" />
      <path d="M9.5 10.6 L26 9.2" stroke="var(--ink-blue)" strokeWidth="2.4" strokeLinecap="round" />
      <path d="M9.5 28.6 L21.5 29.3" stroke="var(--ink-red)" strokeWidth="2.4" strokeLinecap="round" />
    </svg>
  );
}

export function Logo({ big = false }: { big?: boolean }) {
  return (
    <span className={big ? "logo logo--big" : "logo"}>
      <LogoMark size={big ? 64 : 34} />
      <span className="logo__word">завертыш</span>
    </span>
  );
}

const AVATAR_HUES = [8, 32, 48, 145, 190, 222, 262, 320];

export function Avatar({ name, size = 36 }: { name: string; size?: number }) {
  const hash = [...name].reduce((h, c) => (h * 31 + c.charCodeAt(0)) >>> 0, 7);
  const hue = AVATAR_HUES[hash % AVATAR_HUES.length];
  return (
    <span
      className="avatar"
      style={{ width: size, height: size, fontSize: size * 0.42, ["--hue" as string]: hue }}
      aria-hidden="true"
    >
      {name.slice(0, 1).toUpperCase()}
    </span>
  );
}

/** Avatars of several players overlapping each other, the rest as "+N". */
export function Avatars({ names, size = 30, max = 4 }: { names: string[]; size?: number; max?: number }) {
  const shown = names.length > max ? names.slice(0, max - 1) : names;
  return (
    <span className="avatars">
      {shown.map((name) => (
        <Avatar key={name} name={name} size={size} />
      ))}
      {shown.length < names.length && <span className="avatars__more">+{names.length - shown.length}</span>}
    </span>
  );
}

export function Shell({ children }: { children: ReactNode }) {
  const { auth, logout } = useAuth();
  return (
    <div className="shell">
      <header className="topbar">
        <a href={links.lobby} className="topbar__home" aria-label="В лобби">
          <Logo />
        </a>
        {auth && (
          <div className="topbar__user">
            <ConnectionBadge />
            <Avatar name={auth.name} size={32} />
            <span className="topbar__name">{auth.name}</span>
            <button className="btn btn--ghost btn--small" onClick={logout}>
              Выйти
            </button>
          </div>
        )}
      </header>
      <main className="shell__main">{children}</main>
      <footer className="footer">Завертыш · игра для компании, бумага и немного абсурда</footer>
    </div>
  );
}

const CONNECTION_LABELS: Record<ConnectionStatus, string> = {
  online: "на связи",
  connecting: "подключаемся…",
  offline: "нет связи",
};

/** Whether the opponent's moves arrive live. */
function ConnectionBadge() {
  const { status } = useConnection();
  return (
    <span
      className={`connection connection--${status}`}
      title={status === "online" ? "Ходы других игроков приходят сразу" : "Переподключаемся, ходы пока обновляются реже"}
    >
      <i aria-hidden="true" />
      <span className="connection__label">{CONNECTION_LABELS[status]}</span>
    </span>
  );
}

/** The folded part of a sheet: one strip per folded question or answer. */
export function FoldedStrips({ count, max = 8 }: { count: number; max?: number }) {
  if (count === 0) return null;
  const visible = Math.min(count, max);
  return (
    <div className="strips" aria-label={`Загнуто: ${count}`}>
      {Array.from({ length: visible }, (_, i) => (
        <span key={i} className="strip" style={{ ["--i" as string]: i }} />
      ))}
      {count > max && <span className="strips__more">+{count - max}</span>}
    </div>
  );
}

/** A pencil scribbling on paper, shown while the opponent is busy. */
export function Scribble({ label }: { label: ReactNode }) {
  return (
    <div className="scribble">
      <svg viewBox="0 0 220 70" className="scribble__svg" aria-hidden="true">
        <path
          className="scribble__line"
          d="M8 44 C 24 20, 34 58, 50 36 S 78 22, 92 42 S 120 60, 136 34 S 166 18, 182 40 S 204 50, 212 30"
        />
      </svg>
      <svg viewBox="0 0 64 64" className="scribble__pencil" aria-hidden="true">
        <g transform="rotate(-38 32 32)">
          <rect x="22" y="4" width="20" height="40" rx="3" fill="var(--pencil)" />
          <rect x="22" y="4" width="20" height="8" rx="3" fill="var(--ink-red)" />
          <rect x="22" y="11" width="20" height="4" fill="#c9c2b3" />
          <path d="M22 44 L42 44 L32 60 Z" fill="#e8c9a0" />
          <path d="M29 55 L35 55 L32 60 Z" fill="var(--ink)" />
        </g>
      </svg>
      <p className="scribble__label">
        {label}
        <span className="dots" aria-hidden="true">
          <i />
          <i />
          <i />
        </span>
      </p>
    </div>
  );
}

export function Spinner() {
  return <span className="spinner" aria-label="Загрузка" />;
}
