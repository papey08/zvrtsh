import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { api, call, humanize, type Invite, type Session } from "../api/client";
import { Avatar, Spinner } from "../components/ui";
import { useMe } from "../lib/auth";
import {
  badgeOf,
  copyText,
  creatorOf,
  myDoneActions,
  opponentOf,
  plural,
  timeAgo,
} from "../lib/game";
import { newer, useConnection, useSessionEvents } from "../lib/events";
import { inviteCodeFrom, links, navigate } from "../lib/router";
import { useToast } from "../lib/toast";
import { usePolling } from "../lib/usePolling";

/**
 * Merges new snapshots into the list keeping the newest version of every
 * session. With replace the list is limited to the incoming sessions.
 */
function mergeSessions(list: Session[], incoming: Session[], replace: boolean): Session[] {
  const byId = new Map(list.map((s) => [s.id, s]));
  const result = new Map(replace ? [] : byId);
  for (const s of incoming) {
    result.set(s.id, newer(byId.get(s.id) ?? null, s));
  }
  return [...result.values()].sort((a, b) => Date.parse(b.updated_at) - Date.parse(a.updated_at));
}

export function LobbyScreen({ initialCode }: { initialCode?: string }) {
  const me = useMe();
  const toast = useToast();
  const [sessions, setSessions] = useState<Session[] | null>(null);
  const [invite, setInvite] = useStoredInvite(me);
  const { status, syncKey } = useConnection();
  const known = useRef<Set<string> | null>(null); // null until the first load

  /** Takes sessions from the server: the whole list (replace) or single updates. */
  const receive = useCallback(
    (incoming: Session[], replace: boolean) => {
      const seen = known.current;
      // somebody has used our invite: jump straight into the new game
      const fresh = seen && invite && incoming.find((s) => !seen.has(s.id) && creatorOf(s) !== me);
      known.current = new Set([...(replace ? [] : (seen ?? [])), ...incoming.map((s) => s.id)]);
      setSessions((list) => mergeSessions(list ?? [], incoming, replace));
      if (fresh) {
        setInvite(null);
        toast(`${opponentOf(fresh, me)} принимает приглашение!`, "success");
        navigate(links.game(fresh.id));
      }
    },
    [me, invite, setInvite, toast],
  );

  const load = useCallback(async () => receive(await call(api.GET("/sessions")), true), [receive]);

  // the list is loaded once and after every reconnect, then it is kept up to date by the events
  useEffect(() => {
    load().catch(() => {});
  }, [load, syncKey]);
  useSessionEvents((session) => receive([session], false));
  // without the socket the lobby falls back to polling
  usePolling(load, 5000, status === "offline");

  const active = sessions?.filter((s) => s.status !== "done") ?? [];
  const finished = sessions?.filter((s) => s.status === "done") ?? [];

  return (
    <div className="lobby">
      <section className="lobby__hello">
        <h1 className="page-title">
          Привет, <span className="hand hand--yellow">{me}</span>
        </h1>
        <p className="page-subtitle">Позовите соперника по коду или примите его приглашение.</p>
      </section>

      <div className="lobby__actions">
        <InviteCard invite={invite} onInvite={setInvite} />
        <JoinCard initialCode={initialCode} />
      </div>

      <section className="lobby__games">
        <h2 className="section-title">
          Мои партии
          {sessions && sessions.length > 0 && <span className="section-title__count">{sessions.length}</span>}
        </h2>
        {sessions === null ? (
          <div className="empty">
            <Spinner />
          </div>
        ) : sessions.length === 0 ? (
          <div className="empty">
            <div className="empty__sheet" aria-hidden="true">
              <span />
              <span />
              <span />
            </div>
            <p>Партий пока нет. Создайте партию и отправьте код другу.</p>
          </div>
        ) : (
          <>
            {active.length > 0 && (
              <div className="games">
                {active.map((s) => (
                  <GameCard key={s.id} session={s} me={me} />
                ))}
              </div>
            )}
            {finished.length > 0 && (
              <>
                <h3 className="section-subtitle">Завершённые</h3>
                <div className="games">
                  {finished.map((s) => (
                    <GameCard key={s.id} session={s} me={me} />
                  ))}
                </div>
              </>
            )}
          </>
        )}
        <p className="lobby__note">Партия хранится сутки после последнего хода.</p>
      </section>
    </div>
  );
}

const STEP_PRESETS = [3, 5, 7, 10];

function InviteCard({ invite, onInvite }: { invite: Invite | null; onInvite(invite: Invite | null): void }) {
  const toast = useToast();
  const [steps, setSteps] = useState(5);
  const [busy, setBusy] = useState(false);
  const left = useCountdown(invite?.expires_at);

  useEffect(() => {
    if (invite && left <= 0) onInvite(null);
  }, [invite, left, onInvite]);

  async function create(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    try {
      onInvite(await call(api.POST("/invites", { body: { steps_amount: steps } })));
    } catch (err) {
      toast(humanize(err), "error");
    } finally {
      setBusy(false);
    }
  }

  // the invite is cancelled on the server, so the old code stops working
  async function startOver() {
    setBusy(true);
    try {
      await call(api.DELETE("/invites"));
      onInvite(null);
    } catch (err) {
      toast(humanize(err), "error");
    } finally {
      setBusy(false);
    }
  }

  async function copy(text: string, done: string) {
    const ok = await copyText(text);
    toast(ok ? done : "Не получилось скопировать", ok ? "success" : "error");
  }

  const link = invite ? `${window.location.origin}${window.location.pathname}${links.join(invite.invite_code)}` : "";

  return (
    <section className="card action-card action-card--invite">
      <div className="action-card__icon" aria-hidden="true">
        ✉
      </div>
      <h2 className="action-card__title">Позвать соперника</h2>
      {invite ? (
        <>
          <p className="action-card__text">
            Партия на {invite.steps_amount} {plural(invite.steps_amount, ["шаг", "шага", "шагов"])}. Отправьте код или
            ссылку — игра откроется сама, как только соперник присоединится.
          </p>
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
          <div className="action-card__row">
            <button className="btn btn--primary" onClick={() => copy(link, "Ссылка скопирована")}>
              Скопировать ссылку
            </button>
            <button className="btn btn--ghost" onClick={startOver} disabled={busy}>
              Новая партия
            </button>
          </div>
          <p className="action-card__meta">
            <span className="pulse" aria-hidden="true" /> Ждём соперника · код действует ещё {formatLeft(left)}
          </p>
        </>
      ) : (
        <form className="action-card__form" onSubmit={create}>
          <p className="action-card__text">Выберите длину партии и получите код, который нужно передать другу.</p>
          <div className="field">
            <span className="field__label">
              Шагов в партии: <b className="steps-value">{steps}</b>
            </span>
            <div className="steps-picker">
              {STEP_PRESETS.map((n) => (
                <button
                  key={n}
                  type="button"
                  className={n === steps ? "chip is-active" : "chip"}
                  onClick={() => setSteps(n)}
                >
                  {n}
                </button>
              ))}
              <input
                className="range"
                type="range"
                min={1}
                max={50}
                value={steps}
                onChange={(e) => setSteps(Number(e.target.value))}
                aria-label="Количество шагов"
              />
            </div>
          </div>
          <button className="btn btn--primary" disabled={busy}>
            {busy ? "Создаём…" : "Создать партию"}
          </button>
        </form>
      )}
    </section>
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

function JoinCard({ initialCode }: { initialCode?: string }) {
  const [code, setCode] = useState(initialCode ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (initialCode) setCode(initialCode);
  }, [initialCode]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const session = await call(api.POST("/sessions", { body: { invite_code: inviteCodeFrom(code) } }));
      navigate(links.game(session.id));
    } catch (err) {
      setError(humanize(err));
      setBusy(false);
    }
  }

  return (
    <form className={`card action-card${initialCode ? " is-highlighted" : ""}`} onSubmit={submit}>
      <div className="action-card__icon" aria-hidden="true">
        ✎
      </div>
      <h2 className="action-card__title">{initialCode ? "Вас пригласили!" : "Есть код?"}</h2>
      <label className="field">
        <span className="field__label">Код или ссылка-приглашение</span>
        <input
          className="input input--mono"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
          spellCheck={false}
          required
        />
      </label>
      {error && <p className="form-error">{error}</p>}
      <button className="btn btn--primary" disabled={busy || !inviteCodeFrom(code)}>
        {busy ? "Присоединяемся…" : "Присоединиться к партии"}
      </button>
    </form>
  );
}

function GameCard({ session, me }: { session: Session; me: string }) {
  const opponent = opponentOf(session, me);
  const badge = badgeOf(session);
  const done = myDoneActions(session);
  const total = session.steps_amount * 2;

  return (
    <a className={`game-card game-card--${badge.tone}`} href={links.game(session.id)}>
      <div className="game-card__players">
        <Avatar name={me} size={30} />
        <span className="game-card__vs">×</span>
        <Avatar name={opponent} size={30} />
        <span className="game-card__name">{opponent}</span>
      </div>
      <span className={`badge badge--${badge.tone}`}>{badge.label}</span>
      <div className="game-card__progress" aria-label={`Сделано ${done} из ${total}`}>
        <span style={{ width: `${(done / total) * 100}%` }} />
      </div>
      <div className="game-card__meta">
        <span>
          {session.steps_amount} {plural(session.steps_amount, ["шаг", "шага", "шагов"])}
        </span>
        <span>{timeAgo(session.updated_at)}</span>
      </div>
    </a>
  );
}

/** The active invite survives page reloads until it expires. */
function useStoredInvite(me: string): [Invite | null, (invite: Invite | null) => void] {
  const key = `zvrtsh.invite.${me}`;
  const [invite, setInvite] = useState<Invite | null>(() => {
    try {
      const stored = JSON.parse(localStorage.getItem(key) ?? "null") as Invite | null;
      // invites saved by an older version have no steps_amount
      return stored && typeof stored.steps_amount === "number" && new Date(stored.expires_at).getTime() > Date.now()
        ? stored
        : null;
    } catch {
      return null;
    }
  });

  const update = useCallback(
    (value: Invite | null) => {
      setInvite(value);
      try {
        if (value) localStorage.setItem(key, JSON.stringify(value));
        else localStorage.removeItem(key);
      } catch {
        // not critical
      }
    },
    [key],
  );
  return [invite, update];
}

function useCountdown(until: string | undefined): number {
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!until) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [until]);
  return until ? Math.max(0, Math.floor((new Date(until).getTime() - now) / 1000)) : 0;
}

function formatLeft(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}
