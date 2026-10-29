import type { Session } from "../api/client";

/** What the player has to do next in the session. */
export type Stage =
  | { kind: "write"; step: number }
  | { kind: "wait-question"; step: number }
  | { kind: "answer"; step: number }
  | { kind: "wait-finish" }
  | { kind: "done" };

/**
 * At every step the player writes their own question and then answers the
 * opponent's one. The question of step i+1 opens after the answer of step i.
 */
export function stageOf(session: Session): Stage {
  if (session.status === "done") return { kind: "done" };
  for (let step = 0; step < session.progress.length; step++) {
    const p = session.progress[step];
    if (!p.my_question) return { kind: "write", step };
    if (!p.my_answer) return p.opponent_question ? { kind: "answer", step } : { kind: "wait-question", step };
  }
  return { kind: "wait-finish" };
}

export function opponentOf(session: Session, me: string): string {
  return session.players.find((p) => p !== me) ?? session.players[1];
}

/** The player who created the session (used the invite). */
export function creatorOf(session: Session): string {
  return session.players[0];
}

export function myDoneActions(session: Session): number {
  return session.progress.reduce((n, p) => n + Number(p.my_question) + Number(p.my_answer), 0);
}

export function opponentDoneActions(session: Session): number {
  return session.progress.reduce((n, p) => n + Number(p.opponent_question) + Number(p.opponent_answer), 0);
}

export type Badge = { label: string; tone: "turn" | "wait" | "done" };

/**
 * The step the session ends with if both players agree to finish early. It
 * mirrors Session.CurrentStep of the backend: the latest step anybody has
 * written on, or the first unfinished one if it is later.
 */
export function currentStepOf(session: Session): number {
  let touched = 0;
  session.progress.forEach((p, i) => {
    if (p.my_question || p.opponent_question) touched = i;
  });
  const unfinished = session.progress.findIndex((p) => !(p.my_answer && p.opponent_answer));
  return unfinished === -1 ? touched : Math.max(touched, unfinished);
}

export function badgeOf(session: Session): Badge {
  const stage = stageOf(session);
  const canFinishEarly = stage.kind !== "done" && currentStepOf(session) + 1 < session.steps_amount;
  if (canFinishEarly && session.opponent_finish_vote && !session.my_finish_vote) {
    return { label: "Предлагают закончить", tone: "turn" };
  }
  switch (stage.kind) {
    case "done":
      return { label: "Можно разворачивать", tone: "done" };
    case "write":
    case "answer":
      return { label: "Ваш ход", tone: "turn" };
    default:
      return { label: "Ход соперника", tone: "wait" };
  }
}

/** What the opponent is busy with while the player waits for their question. */
export function opponentActivity(session: Session, step: number): string {
  const previous = session.progress[step - 1];
  if (previous && !previous.opponent_answer) return "отвечает на ваш вопрос";
  return "сочиняет вопрос";
}

export function plural(n: number, forms: [string, string, string]): string {
  const mod10 = n % 10;
  const mod100 = n % 100;
  if (mod10 === 1 && mod100 !== 11) return forms[0];
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return forms[1];
  return forms[2];
}

export function timeAgo(iso: string, now = Date.now()): string {
  const minutes = Math.floor((now - new Date(iso).getTime()) / 60000);
  if (minutes < 1) return "только что";
  if (minutes < 60) return `${minutes} ${plural(minutes, ["минуту", "минуты", "минут"])} назад`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} ${plural(hours, ["час", "часа", "часов"])} назад`;
  return new Date(iso).toLocaleDateString("ru-RU", { day: "numeric", month: "long" });
}

/**
 * Guesses the visible part of a question: its first word, e.g. "Что?", or two
 * words if the first one is too short to mean anything alone, e.g. "С кем?".
 */
export function suggestOpenPart(question: string): string {
  const words = question.trim().split(/[\s,.!?…:;—-]+/).filter(Boolean);
  if (words.length === 0) return "";
  const open = words.slice(0, words[0].length < 3 ? 2 : 1).join(" ");
  return open[0].toUpperCase() + open.slice(1) + "?";
}

/** Every player writes in their own ink: the creator of the session in blue, the other one in red. */
export function inkOf(session: Session, player: string): string {
  return player === creatorOf(session) ? "ink--1" : "ink--2";
}

export const QUESTION_WORDS = ["Кто?", "Что?", "Где?", "Когда?", "Зачем?", "Как?", "Сколько?", "С кем?", "Какой?"];

const RANDOM_ANSWERS = [
  "Пуговицы",
  "В субботу",
  "Бабушка",
  "На Луне",
  "Потому что",
  "Три раза",
  "Вчерашний борщ",
  "С енотом",
  "Под диваном",
  "Очень громко",
  "Никогда",
  "Директор цирка",
  "Тайком",
  "Сорок два",
  "Зелёный",
  "В трамвае",
];

export function randomAnswer(except = ""): string {
  const options = RANDOM_ANSWERS.filter((a) => a !== except);
  return options[Math.floor(Math.random() * options.length)];
}

export const MAX_TEXT = 500;

export function validateName(name: string): string | null {
  if (name.length < 4 || name.length > 30) return "Имя — от 4 до 30 символов";
  if (!/^[A-Za-z0-9_]+$/.test(name)) return "В имени только латиница, цифры и _";
  return null;
}

export function validatePassword(password: string): string | null {
  if (password.length < 4 || password.length > 30) return "Пароль — от 4 до 30 символов";
  if (!/^[A-Za-z0-9!@#$%^&*()\-_=+[\]{}\\|/'";:,.<>?]+$/.test(password)) {
    return "В пароле только латиница, цифры и знаки";
  }
  return null;
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // clipboard API needs a secure context, fall back to the old way
    const area = document.createElement("textarea");
    area.value = text;
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    const ok = document.execCommand("copy");
    area.remove();
    return ok;
  }
}
