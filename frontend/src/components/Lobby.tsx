import { useEffect, useState } from "react";
import type { Invite, Session } from "../api/client";
import { copyText, inkOf, MAX_PLAYERS } from "../lib/game";
import { links } from "../lib/router";
import { useToast } from "../lib/toast";
import { Avatar } from "./ui";

/** The players who have joined the session, each with their ink. */
export function Roster({ session, me }: { session: Session; me: string }) {
  const free = MAX_PLAYERS - session.players.length;
  return (
    <ul className="roster">
      {session.players.map((player) => (
        <li key={player} className={`roster__item ${inkOf(session, player)}`}>
          <Avatar name={player} size={26} />
          <span>{player === me ? "вы" : player}</span>
          {player === session.host && <span className="roster__host">создатель</span>}
        </li>
      ))}
      {free > 0 && <li className="roster__free">ещё {free} {free === 1 ? "место" : free < 5 ? "места" : "мест"}</li>}
    </ul>
  );
}

export function inviteLink(code: string): string {
  return `${window.location.origin}${window.location.pathname}${links.join(code)}`;
}

/** Copies the text and tells the player about it. */
export function useCopy() {
  const toast = useToast();
  return async (text: string, done: string) => {
    const ok = await copyText(text);
    toast(ok ? done : "Не получилось скопировать", ok ? "success" : "error");
  };
}

/** The invite code with a button to copy it, and the time it is valid. */
export function InviteCode({ invite }: { invite: Invite }) {
  const copy = useCopy();
  const left = useCountdown(invite.expires_at);

  return (
    <>
      <div className="code-field">
        <span className="code-field__value">{invite.invite_code}</span>
        <button
          type="button"
          className="code-field__copy"
          onClick={() => copy(invite.invite_code, "Код скопирован")}
          title="Скопировать код"
          aria-label="Скопировать код"
        >
          <CopyIcon />
        </button>
      </div>
      <p className="action-card__meta">
        <span className="pulse" aria-hidden="true" />
        {left > 0 ? `Код действует ещё ${formatLeft(left)}` : "Код устарел — новые игроки уже не войдут"}
      </p>
    </>
  );
}

function CopyIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
      <rect x="9" y="9" width="12" height="12" rx="2.5" />
      <path d="M5 15H4.5A1.5 1.5 0 0 1 3 13.5v-9A1.5 1.5 0 0 1 4.5 3h9A1.5 1.5 0 0 1 15 4.5V5" />
    </svg>
  );
}

function useCountdown(until: string): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [until]);
  return Math.max(0, Math.floor((new Date(until).getTime() - now) / 1000));
}

function formatLeft(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}
