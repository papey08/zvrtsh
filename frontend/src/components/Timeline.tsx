import type { Session } from "../api/client";
import { stageOf } from "../lib/game";
import { Avatar } from "./ui";

/**
 * Progress of the player and of the whole company: every step has a question
 * half and an answer half, the company's halves fill up as players finish them.
 */
export function Timeline({ session, me }: { session: Session; me: string }) {
  const stage = stageOf(session);
  const current = "step" in stage ? stage.step : -1;
  const n = session.players.length;

  const rows = [
    {
      who: "Вы",
      avatar: <Avatar name={me} size={24} />,
      marks: session.progress.map((p) => [Number(p.my_question), Number(p.my_answer)]),
    },
    {
      who: "Все",
      avatar: <span className="timeline__all">{n}</span>,
      marks: session.progress.map((p) => [p.questions_written / n, p.answers_written / n]),
    },
  ];

  return (
    <div className="timeline" style={{ ["--steps" as string]: session.steps_amount }}>
      {rows.map((row) => (
        <div className="timeline__row" key={row.who}>
          <span className="timeline__who">
            {row.avatar}
            <span>{row.who}</span>
          </span>
          <span className="timeline__cells">
            {row.marks.map(([question, answer], i) => (
              <span
                key={i}
                className={`timeline__cell${i === current ? " is-current" : ""}`}
                title={`Шаг ${i + 1}: вопросы ${Math.round(question * n)} из ${n}, ответы ${Math.round(answer * n)} из ${n}`}
              >
                <i className={question === 1 ? "is-on" : ""} style={{ ["--fill" as string]: question }} />
                <i className={answer === 1 ? "is-on" : ""} style={{ ["--fill" as string]: answer }} />
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
