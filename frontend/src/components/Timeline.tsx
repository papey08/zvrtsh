import type { Session } from "../api/client";
import { opponentOf, stageOf } from "../lib/game";
import { Avatar } from "./ui";

/** Progress of both players: every step has a question half and an answer half. */
export function Timeline({ session, me }: { session: Session; me: string }) {
  const opponent = opponentOf(session, me);
  const stage = stageOf(session);
  const current = "step" in stage ? stage.step : -1;

  const rows = [
    { who: "Вы", name: me, marks: session.progress.map((p) => [p.my_question, p.my_answer]) },
    { who: opponent, name: opponent, marks: session.progress.map((p) => [p.opponent_question, p.opponent_answer]) },
  ];

  return (
    <div className="timeline" style={{ ["--steps" as string]: session.steps_amount }}>
      {rows.map((row) => (
        <div className="timeline__row" key={row.who}>
          <span className="timeline__who">
            <Avatar name={row.name} size={24} />
            <span>{row.who}</span>
          </span>
          <span className="timeline__cells">
            {row.marks.map(([question, answer], i) => (
              <span
                key={i}
                className={`timeline__cell${i === current ? " is-current" : ""}`}
                title={`Шаг ${i + 1}: ${question ? "вопрос написан" : "вопроса нет"}, ${answer ? "ответ дан" : "ответа нет"}`}
              >
                <i className={question ? "is-on" : ""} />
                <i className={answer ? "is-on" : ""} />
              </span>
            ))}
          </span>
        </div>
      ))}
      <div className="timeline__legend">
        <span>
          <i className="is-on" /> вопрос
        </span>
        <span>
          <i className="is-on is-answer" /> ответ
        </span>
      </div>
    </div>
  );
}
