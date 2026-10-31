import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { api, call, humanize, type Session } from "../api/client";
import { InviteCode, inviteLink, Roster, useCopy } from "../components/Lobby";
import { Avatars, Spinner } from "../components/ui";
import { useMe } from "../lib/auth";
import { badgeOf, joinNames, MAX_PLAYERS, MAX_STEPS, myDoneActions, othersOf, plural, timeAgo } from "../lib/game";
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
  const { status, syncKey } = useConnection();
  const statuses = useRef(new Map<string, Session["status"]>());

  /** Takes sessions from the server: the whole list (replace) or single updates. */
  const receive = useCallback(
    (incoming: Session[], replace: boolean) => {
      // the host has started a session we wait in: jump straight into it
      const started = incoming.find(
        (s) => statuses.current.get(s.id) === "lobby" && s.status === "in_work" && s.host !== me,
      );
      for (const s of incoming) statuses.current.set(s.id, s.status);
      setSessions((list) => mergeSessions(list ?? [], incoming, replace));
      if (started) {
        toast(`${started.host} начинает партию!`, "success");
        navigate(links.game(started.id));
      }
    },
    [me, toast],
  );

  const load = useCallback(async () => receive(await call(api.GET("/sessions")), true), [receive]);

  // the list is loaded once and after every reconnect, then it is kept up to date by the events
  useEffect(() => {
    load().catch(() => {});
  }, [load, syncKey]);
  useSessionEvents(
    (session) => receive([session], false),
    (id) => setSessions((list) => list?.filter((s) => s.id !== id) ?? null),
  );
  // without the socket the lobby falls back to polling
  usePolling(load, 5000, status === "offline");

  // the session the player has created and still waits in
  const hosted = sessions?.find((s) => s.status === "lobby" && s.host === me) ?? null;
  const active = sessions?.filter((s) => s.status !== "done" && s !== hosted) ?? [];
  const finished = sessions?.filter((s) => s.status === "done") ?? [];
  const count = (sessions?.length ?? 0) - (hosted ? 1 : 0);

  return (
    <div className="lobby">
      <section className="lobby__hello">
        <h1 className="page-title">
          Привет, <span className="hand hand--yellow">{me}</span>
        </h1>
        <p className="page-subtitle">Соберите компанию до {MAX_PLAYERS} человек по коду или присоединитесь к чужой партии.</p>
      </section>

      <div className="lobby__actions">
        <InviteCard lobby={hosted} loading={sessions === null} onChanged={load} />
        <JoinCard initialCode={initialCode} />
      </div>

      <section className="lobby__games">
        <h2 className="section-title">
          Мои партии
          {count > 0 && <span className="section-title__count">{count}</span>}
        </h2>
        {sessions === null ? (
          <div className="empty">
            <Spinner />
          </div>
        ) : count === 0 ? (
          <div className="empty">
            <div className="empty__sheet" aria-hidden="true">
              <span />
              <span />
              <span />
            </div>
            <p>Партий пока нет. Создайте партию и отправьте код друзьям.</p>
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

type InviteCardProps = { lobby: Session | null; loading: boolean; onChanged(): Promise<void> };

function InviteCard({ lobby, loading, onChanged }: InviteCardProps) {
  const me = useMe();
  const toast = useToast();
  const copy = useCopy();
  const [steps, setSteps] = useState(5);
  const [busy, setBusy] = useState(false);

  async function act(request: () => Promise<unknown>) {
    setBusy(true);
    try {
      await request();
      await onChanged();
    } catch (err) {
      toast(humanize(err), "error");
    } finally {
      setBusy(false);
    }
  }

  const create = (e: FormEvent) => {
    e.preventDefault();
    void act(() => call(api.POST("/invites", { body: { steps_amount: steps } })));
  };
  // the session is cancelled on the server, so the old code stops working
  const startOver = () => act(() => call(api.DELETE("/invites")));

  async function start(id: string) {
    setBusy(true);
    try {
      await call(api.POST("/sessions/{session_id}/start", { params: { path: { session_id: id } } }));
      navigate(links.game(id));
    } catch (err) {
      toast(humanize(err), "error");
      setBusy(false);
    }
  }

  const enough = lobby !== null && lobby.players.length > 1;

  return (
    <section className="card action-card action-card--invite">
      <div className="action-card__icon" aria-hidden="true">
        ✉
      </div>
      <h2 className="action-card__title">Собрать компанию</h2>
      {lobby ? (
        <>
          <p className="action-card__text">
            Партия на {lobby.steps_amount} {plural(lobby.steps_amount, ["шаг", "шага", "шагов"])}. Отправьте код или
            ссылку друзьям и начните, когда все соберутся.
          </p>
          {lobby.invite && <InviteCode invite={lobby.invite} />}
          <div className="action-card__row">
            {lobby.invite && (
              <button className="btn btn--ghost" onClick={() => copy(inviteLink(lobby.invite!.invite_code), "Ссылка скопирована")}>
                Скопировать ссылку
              </button>
            )}
            <button className="btn btn--ghost" onClick={startOver} disabled={busy}>
              Новая партия
            </button>
          </div>
          <div className="field">
            <span className="field__label">
              В партии {lobby.players.length} из {MAX_PLAYERS}
            </span>
            <Roster session={lobby} me={me} />
          </div>
          <button className="btn btn--primary" onClick={() => start(lobby.id)} disabled={busy || !enough}>
            Начать партию
          </button>
          {!enough && <p className="action-card__hint">Начать можно, когда присоединится хотя бы один игрок.</p>}
        </>
      ) : (
        <form className="action-card__form" onSubmit={create}>
          <p className="action-card__text">Выберите длину партии и получите код, который нужно передать друзьям.</p>
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
                max={MAX_STEPS}
                value={steps}
                onChange={(e) => setSteps(Number(e.target.value))}
                aria-label="Количество шагов"
              />
            </div>
          </div>
          <button className="btn btn--primary" disabled={busy || loading}>
            {busy ? "Создаём…" : "Создать партию"}
          </button>
        </form>
      )}
    </section>
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
  const others = othersOf(session, me);
  const badge = badgeOf(session, me);
  const done = myDoneActions(session);
  const total = session.steps_amount * 2;

  return (
    <a className={`game-card game-card--${badge.tone}`} href={links.game(session.id)}>
      <div className="game-card__head">
        <div className="game-card__players">
          <Avatars names={session.players} />
          <span className="game-card__name">{others.length > 0 ? joinNames(others) : "Пока только вы"}</span>
        </div>
        <span className={`badge badge--${badge.tone}`}>{badge.label}</span>
      </div>
      <div className="game-card__progress" aria-label={`Сделано ${done} из ${total}`}>
        <span style={{ width: `${(done / total) * 100}%` }} />
      </div>
      <div className="game-card__meta">
        <span>
          {session.status === "lobby"
            ? `${session.players.length} ${plural(session.players.length, ["игрок", "игрока", "игроков"])}`
            : `${session.players.length} ${plural(session.players.length, ["игрок", "игрока", "игроков"])} · ${session.steps_amount} ${plural(session.steps_amount, ["шаг", "шага", "шагов"])}`}
        </span>
        <span>{timeAgo(session.updated_at)}</span>
      </div>
    </a>
  );
}
