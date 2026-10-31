import { useEffect, useState } from "react";
import { api, call, humanize, type Results, type Session } from "../api/client";
import { Avatar, Spinner } from "../components/ui";
import { copyText, inkOf as inkIn } from "../lib/game";
import { links } from "../lib/router";
import { useToast } from "../lib/toast";

type Sheet = Results["sheets"][number];

export function ResultsView({ session, me }: { session: Session; me: string }) {
  const toast = useToast();
  const [results, setResults] = useState<Results | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [unfolded, setUnfolded] = useState(false);

  useEffect(() => {
    call(api.GET("/sessions/{session_id}/results", { params: { path: { session_id: session.id } } }))
      .then(setResults)
      .catch((err) => setError(humanize(err)));
  }, [session.id]);

  if (error) return <p className="form-error">{error}</p>;
  if (!results) {
    return (
      <div className="empty">
        <Spinner />
      </div>
    );
  }

  // the own sheet goes first
  const sheets = [...results.sheets].sort((a, b) => Number(b.owner === me) - Number(a.owner === me));

  if (!unfolded) {
    return (
      <div className="reveal">
        <div className="reveal__stack" aria-hidden="true">
          {["a", "b"].map((side) => (
            <div key={side} className={`paper reveal__sheet reveal__sheet--${side}`}>
              {Array.from({ length: Math.min(session.steps_amount * 2, 10) }, (_, i) => (
                <span key={i} className="strip" style={{ ["--i" as string]: i }} />
              ))}
            </div>
          ))}
        </div>
        <h2 className="reveal__title">Все листы загнуты до конца</h2>
        <p className="page-subtitle">
          {results.sheets.length === 2 ? "Листов два" : `Листов ${results.sheets.length}`} — самое время узнать, что
          вы насочиняли.
        </p>
        <button className="btn btn--primary btn--huge" onClick={() => setUnfolded(true)}>
          Развернуть листы
        </button>
      </div>
    );
  }

  async function copy() {
    const ok = await copyText(resultsToText(sheets));
    toast(ok ? "Результаты скопированы" : "Не получилось скопировать", ok ? "success" : "error");
  }

  const inkOf = (player: string) => inkIn(session, player);

  return (
    <div className="results">
      <div className="results__legend">
        {session.players.map((player) => (
          <span key={player} className={`results__legend-item ${inkOf(player)}`}>
            <i aria-hidden="true" />
            {player === me ? "вы" : player}
          </span>
        ))}
      </div>
      <div className="results__sheets">
        {sheets.map((sheet, i) => (
          <ResultSheet key={sheet.owner} sheet={sheet} me={me} inkOf={inkOf} delay={Math.min(i, 3) * 0.25} />
        ))}
      </div>
      <div className="results__actions">
        <button className="btn btn--ghost" onClick={copy}>
          Скопировать текстом
        </button>
        <a className="btn btn--primary" href={links.lobby}>
          Сыграть ещё
        </a>
      </div>
    </div>
  );
}

type ResultSheetProps = {
  sheet: Sheet;
  me: string;
  inkOf(player: string): string;
  delay: number;
};

function ResultSheet({ sheet, me, inkOf, delay }: ResultSheetProps) {
  return (
    <article className="paper result-sheet">
      <header className="result-sheet__head">
        <Avatar name={sheet.owner} size={30} />
        <h3>{sheet.owner === me ? "Ваш лист" : `Лист ${sheet.owner}`}</h3>
      </header>
      <ol className="result-sheet__steps">
        {sheet.steps.map((step, i) => {
          const { asker, answerer } = step;
          return (
            <li
              key={i}
              className="result-step"
              style={{ animationDelay: `${delay + i * 0.35}s` }}
            >
              <p className={`result-step__q ${inkOf(asker)}`}>{step.hidden_question}</p>
              <p className={`result-step__a ${inkOf(answerer)}`}>{step.answer}</p>
              <p className="result-step__meta">
                вопрос: {asker === me ? "ваш" : asker} · было видно «{step.open_question}» · ответ:{" "}
                {answerer === me ? "ваш" : answerer}
              </p>
            </li>
          );
        })}
      </ol>
    </article>
  );
}

function resultsToText(sheets: Sheet[]): string {
  const lines = [`Завертыш: ${sheets.map((s) => s.owner).join(" × ")}`];
  for (const sheet of sheets) {
    lines.push("", `Лист ${sheet.owner}`);
    sheet.steps.forEach((step, i) => lines.push(`${i + 1}. ${step.hidden_question} — ${step.answer}`));
  }
  return lines.join("\n");
}
