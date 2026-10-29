import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import { api, ApiError, call, humanize, type Session } from "../api/client";
import { ResultsView } from "./ResultsView";
import { Timeline } from "../components/Timeline";
import { Avatar, FoldedStrips, Scribble, Spinner } from "../components/ui";
import { useMe } from "../lib/auth";
import {
  MAX_TEXT,
  currentStepOf,
  inkOf,
  QUESTION_WORDS,
  myDoneActions,
  opponentActivity,
  opponentDoneActions,
  opponentOf,
  plural,
  randomAnswer,
  sleep,
  stageOf,
  suggestOpenPart,
  type Stage,
} from "../lib/game";
import { newer, useConnection, useSessionEvents } from "../lib/events";
import { links } from "../lib/router";
import { useToast } from "../lib/toast";
import { usePolling } from "../lib/usePolling";

const FOLD_MS = 750;

export function GameScreen({ id }: { id: string }) {
  const me = useMe();
  const [session, setSession] = useState<Session | null>(null);
  const [fatal, setFatal] = useState<string | null>(null);

  const { status, syncKey } = useConnection();

  const load = useCallback(async () => {
    try {
      const next = await call(api.GET("/sessions/{session_id}", { params: { path: { session_id: id } } }));
      setSession((current) => newer(current, next));
    } catch (err) {
      if (err instanceof ApiError && (err.status === 403 || err.status === 404)) setFatal(humanize(err));
      throw err;
    }
  }, [id]);

  // the session is loaded once and after every reconnect, the opponent's moves come as events
  useEffect(() => {
    load().catch(() => {});
  }, [load, syncKey]);
  useSessionEvents((next) => {
    if (next.id === id) setSession((current) => newer(current, next));
  });

  // the players have agreed to finish early: the session became shorter
  const toast = useToast();
  const stepsAmount = session?.steps_amount;
  const previousSteps = useRef(stepsAmount);
  useEffect(() => {
    if (stepsAmount && previousSteps.current && stepsAmount < previousSteps.current) {
      toast(`Договорились! Листы развернутся после шага ${stepsAmount}`, "success");
    }
    previousSteps.current = stepsAmount;
  }, [stepsAmount, toast]);

  const stage = session ? stageOf(session) : null;
  // without the socket the game falls back to polling
  usePolling(load, 3000, status === "offline" && !fatal && stage?.kind !== "done");

  if (fatal) {
    return (
      <div className="game-missing card">
        <h1 className="page-title">Упс</h1>
        <p className="page-subtitle">{fatal}</p>
        <a className="btn btn--primary" href={links.lobby}>
          В лобби
        </a>
      </div>
    );
  }

  if (!session || !stage) {
    return (
      <div className="empty">
        <Spinner />
      </div>
    );
  }

  const opponent = opponentOf(session, me);
  // on the last step there is nothing to cut
  const canFinishEarly = stage.kind !== "done" && currentStepOf(session) + 1 < session.steps_amount;

  return (
    <div className="game">
      <header className="game__head">
        <a className="back" href={links.lobby}>
          ← Лобби
        </a>
        <div className="game__players">
          <span className="player">
            <Avatar name={me} size={34} />
            <span>Вы</span>
          </span>
          <span className="game__vs">×</span>
          <span className="player">
            <Avatar name={opponent} size={34} />
            <span>{opponent}</span>
          </span>
        </div>
        <span className="game__step">
          {"step" in stage
            ? `Шаг ${stage.step + 1} из ${session.steps_amount}`
            : stage.kind === "done"
              ? "Партия окончена"
              : "Почти всё"}
        </span>
      </header>

      {canFinishEarly && session.opponent_finish_vote && !session.my_finish_vote && (
        <FinishOffer session={session} me={me} onSession={(next) => setSession((current) => newer(current, next))} />
      )}

      {stage.kind === "done" ? (
        <ResultsView session={session} me={me} />
      ) : (
        <div className="game__layout">
          <div className="game__board">
            <StagePanel key={stageKey(stage)} session={session} stage={stage} me={me} onChanged={load} />
          </div>
          <aside className="game__aside">
            <div className="card card--compact">
              <h3 className="aside-title">Ход игры</h3>
              <Timeline session={session} me={me} />
            </div>
            <div className="card card--compact tip">
              <h3 className="aside-title">Подсказка</h3>
              <p>{tipFor(stage)}</p>
            </div>
            {canFinishEarly && !session.opponent_finish_vote && (
              <FinishCard session={session} me={me} onSession={(next) => setSession((current) => newer(current, next))} />
            )}
          </aside>
        </div>
      )}
    </div>
  );
}

type FinishProps = { session: Session; me: string; onSession(session: Session): void };

/** Sends the player's vote to finish the session after the current step. */
function useFinishVote(session: Session, onSession: (session: Session) => void) {
  const toast = useToast();
  const [busy, setBusy] = useState(false);

  async function setVote(vote: boolean) {
    setBusy(true);
    const params = { params: { path: { session_id: session.id } } };
    try {
      const request = vote
        ? api.PUT("/sessions/{session_id}/finish-vote", params)
        : api.DELETE("/sessions/{session_id}/finish-vote", params);
      onSession(await call(request));
    } catch (err) {
      toast(humanize(err), "error");
    } finally {
      setBusy(false);
    }
  }
  return { busy, setVote };
}

/** The player's own offer to finish early, in the side column. */
function FinishCard({ session, me, onSession }: FinishProps) {
  const { busy, setVote } = useFinishVote(session, onSession);
  const opponent = opponentOf(session, me);
  const step = currentStepOf(session) + 1;

  return (
    <div className="card card--compact finish-card">
      <h3 className="aside-title">Надоело?</h3>
      {session.my_finish_vote ? (
        <>
          <p>
            Вы предложили закончить после шага {step}. Ждём, согласится ли <b>{opponent}</b>.
          </p>
          <button className="btn btn--ghost btn--small" onClick={() => setVote(false)} disabled={busy}>
            Передумать
          </button>
        </>
      ) : (
        <>
          <p>Если вы оба согласитесь, доиграете шаг {step} и сразу развернёте листы.</p>
          <button className="btn btn--ghost btn--small" onClick={() => setVote(true)} disabled={busy}>
            Предложить закончить
          </button>
        </>
      )}
    </div>
  );
}

/** The opponent's offer to finish early, shown above the sheet so it is not missed. */
function FinishOffer({ session, me, onSession }: FinishProps) {
  const { busy, setVote } = useFinishVote(session, onSession);
  const step = currentStepOf(session) + 1;

  return (
    <div className="finish-offer">
      <span className="finish-offer__text">
        <b>{opponentOf(session, me)}</b> предлагает закончить после шага {step} и развернуть листы
      </span>
      <button className="btn btn--primary btn--small" onClick={() => setVote(true)} disabled={busy}>
        Согласиться
      </button>
    </div>
  );
}

function stageKey(stage: Stage): string {
  return "step" in stage ? `${stage.kind}-${stage.step}` : stage.kind;
}

function tipFor(stage: Stage): string {
  switch (stage.kind) {
    case "write":
      return "Чем подробнее полный вопрос, тем смешнее финал. Соперник увидит только короткое начало.";
    case "answer":
      return "Не пытайтесь угадать — отвечайте первое, что придёт в голову. Абсурд приветствуется.";
    case "wait-question":
      return "Пока соперник пишет, придумайте следующий вопрос. Страница обновится сама.";
    default:
      return "Листы развернутся, как только соперник закончит последний шаг.";
  }
}

type PanelProps = { session: Session; stage: Stage; me: string; onChanged(): Promise<void> };

function StagePanel(props: PanelProps) {
  const { session, stage, me } = props;
  const opponent = opponentOf(session, me);
  const folded = myDoneActions(session);

  switch (stage.kind) {
    case "write":
      return <WritePanel {...props} step={stage.step} folded={folded} />;
    case "answer":
      return <AnswerPanel {...props} step={stage.step} folded={folded} />;
    case "wait-question":
      return (
        <div className="paper sheet sheet--waiting">
          <FoldedStrips count={folded} />
          <Scribble
            label={
              <>
                <b>{opponent}</b> {opponentActivity(session, stage.step)}
              </>
            }
          />
          <p className="sheet__hint">Ваш вопрос загнут и передан. Ждём вопрос соперника для шага {stage.step + 1}.</p>
        </div>
      );
    default: {
      const left = session.steps_amount * 2 - opponentDoneActions(session);
      return (
        <div className="paper sheet sheet--waiting">
          <FoldedStrips count={folded} />
          <div className="finish-stamp">Готово!</div>
          <Scribble
            label={
              <>
                <b>{opponent}</b> дописывает — осталось {left} {plural(left, ["действие", "действия", "действий"])}
              </>
            }
          />
        </div>
      );
    }
  }
}

function submitOnCtrlEnter(e: KeyboardEvent<HTMLElement>) {
  if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    e.currentTarget.closest("form")?.requestSubmit();
  }
}

type StepProps = PanelProps & { step: number; folded: number };

function WritePanel({ session, step, folded, me, onChanged }: StepProps) {
  // the question mark stays at the end of the field, the player types before it
  const [hidden, setHidden] = useState("");
  const [open, setOpen] = useState("");
  const [openTouched, setOpenTouched] = useState(false);
  const [folding, setFolding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const hiddenRef = useRef<HTMLTextAreaElement>(null);

  function changeHidden(value: string) {
    const text = value.replace(/\?+$/, "");
    setHidden(text);
    if (!openTouched) setOpen(suggestOpenPart(text));
  }

  // keeps the caret and the selection before the question mark
  const keepBeforeMark = useCallback(() => {
    const area = hiddenRef.current;
    if (!area) return;
    const end = area.value.length - 1;
    if (area.selectionEnd > end) area.setSelectionRange(Math.min(area.selectionStart, end), end);
  }, []);
  useLayoutEffect(keepBeforeMark, [hidden, keepBeforeMark]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!hidden.trim() || !open.trim()) {
      setError("Нужны и полный вопрос, и его видимое начало");
      return;
    }
    setError(null);
    setFolding(true);
    try {
      await Promise.all([
        call(
          api.POST("/sessions/{session_id}/steps/{step_index}/question", {
            params: { path: { session_id: session.id, step_index: step } },
            body: { hidden_question: `${hidden.trim()}?`, open_question: open.trim() },
          }),
        ),
        sleep(FOLD_MS),
      ]);
      await onChanged();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        await onChanged();
        return;
      }
      setFolding(false);
      setError(humanize(err));
    }
  }

  return (
    <form className={`paper sheet${folding ? " is-folding" : ""}`} onSubmit={submit}>
      <FoldedStrips count={folded} />
      <div className="sheet__fold-area">
        <p className="sheet__label">
          <span className="sheet__num">{step + 1}</span> Ваш вопрос
        </p>
        <textarea
          ref={hiddenRef}
          className={`paper-input paper-input--area ${inkOf(session, me)}`}
          value={`${hidden}?`}
          onChange={(e) => changeHidden(e.target.value)}
          onSelect={keepBeforeMark}
          onKeyDown={submitOnCtrlEnter}
          maxLength={MAX_TEXT}
          rows={2}
          autoFocus
        />
        <div className="sheet__crease" aria-hidden="true">
          <span>линия сгиба — выше спрячется</span>
        </div>
      </div>

      <p className="sheet__label">Соперник увидит только это</p>
      <input
        className={`paper-input paper-input--open ${inkOf(session, me)}`}
        value={open}
        onChange={(e) => {
          setOpen(e.target.value);
          setOpenTouched(true);
        }}
        onKeyDown={submitOnCtrlEnter}
        placeholder="Что?"
        maxLength={MAX_TEXT}
      />
      <div className="chips">
        {QUESTION_WORDS.map((word) => (
          <button
            key={word}
            type="button"
            className={word === open ? "chip chip--paper is-active" : "chip chip--paper"}
            onClick={() => {
              setOpen(word);
              setOpenTouched(true);
            }}
          >
            {word}
          </button>
        ))}
      </div>

      {error && <p className="form-error form-error--paper">{error}</p>}
      <div className="sheet__actions">
        <button className="btn btn--ink" disabled={folding || !hidden.trim() || !open.trim()}>
          {folding ? "Загибаем…" : "Загнуть и передать"}
        </button>
        <span className="sheet__kbd">⌘/Ctrl + Enter</span>
      </div>
    </form>
  );
}

function AnswerPanel({ session, step, folded, me, onChanged }: StepProps) {
  const opponent = opponentOf(session, me);
  const [question, setQuestion] = useState<string | null>(null);
  const [answer, setAnswer] = useState("");
  const [folding, setFolding] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    call(
      api.GET("/sessions/{session_id}/steps/{step_index}/opponent-question", {
        params: { path: { session_id: session.id, step_index: step } },
      }),
    )
      .then((q) => !cancelled && setQuestion(q.open_question))
      .catch((err) => !cancelled && setError(humanize(err)));
    return () => {
      cancelled = true;
    };
  }, [session.id, step]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!answer.trim()) return;
    setError(null);
    setFolding(true);
    try {
      await Promise.all([
        call(
          api.POST("/sessions/{session_id}/steps/{step_index}/answer", {
            params: { path: { session_id: session.id, step_index: step } },
            body: { answer: answer.trim() },
          }),
        ),
        sleep(FOLD_MS),
      ]);
      await onChanged();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        await onChanged();
        return;
      }
      setFolding(false);
      setError(humanize(err));
    }
  }

  return (
    <form className={`paper sheet${folding ? " is-folding" : ""}`} onSubmit={submit}>
      <FoldedStrips count={folded} />
      <p className="sheet__label">
        <span className="sheet__num">{step + 1}</span> {opponent} спрашивает
      </p>
      <div className={`open-strip ${inkOf(session, opponent)}`}>{question === null && !error ? <Spinner /> : <span>{question}</span>}</div>

      <div className="sheet__fold-area">
        <p className="sheet__label">Ваш ответ вслепую</p>
        <div className="answer-row">
          <input
            className={`paper-input paper-input--answer ${inkOf(session, me)}`}
            value={answer}
            onChange={(e) => setAnswer(e.target.value)}
            placeholder="Пуговицы"
            maxLength={MAX_TEXT}
            autoFocus
          />
          <button
            type="button"
            className="dice"
            onClick={() => setAnswer(randomAnswer(answer))}
            title="Случайный ответ"
            aria-label="Случайный ответ"
          >
            🎲
          </button>
        </div>
      </div>

      {error && <p className="form-error form-error--paper">{error}</p>}
      <div className="sheet__actions">
        <button className="btn btn--ink" disabled={folding || !answer.trim() || question === null}>
          {folding ? "Загибаем…" : "Загнуть ответ"}
        </button>
        <span className="sheet__kbd">Enter</span>
      </div>
    </form>
  );
}
