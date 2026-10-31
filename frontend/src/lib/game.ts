import type { Session } from "../api/client";

/** What the player has to do next in the session. */
export type Stage =
  | { kind: "lobby" }
  | { kind: "write"; step: number }
  | { kind: "answer"; step: number }
  | { kind: "wait-questions"; step: number }
  | { kind: "wait-finish" }
  | { kind: "done" };

/**
 * At every step the player writes a question and then answers the question on
 * another sheet. The sheets are shuffled when everybody has written the
 * question; after the answer the sheet stays with the player for the next
 * question.
 */
export function stageOf(session: Session): Stage {
  if (session.status === "lobby") return { kind: "lobby" };
  if (session.status === "done") return { kind: "done" };
  for (let step = 0; step < session.progress.length; step++) {
    const p = session.progress[step];
    if (!p.my_question) return { kind: "write", step };
    if (!p.my_answer) return p.answer_sheet_ready ? { kind: "answer", step } : { kind: "wait-questions", step };
  }
  return { kind: "wait-finish" };
}

/** The other players of the session, in the order they have joined. */
export function othersOf(session: Session, me: string): string[] {
  return session.players.filter((p) => p !== me);
}

/** "Аня", "Аня и Боря", "Аня, Боря и Вера". */
export function joinNames(names: string[]): string {
  if (names.length <= 1) return names.join("");
  return `${names.slice(0, -1).join(", ")} и ${names[names.length - 1]}`;
}

export const MAX_PLAYERS = 8;
export const MAX_STEPS = 10; // the maximum of CreateInviteRequest in api/openapi.yaml

/** Every player writes in their own ink, by the order of joining. */
export function inkOf(session: Session, player: string): string {
  const idx = session.players.indexOf(player);
  return `ink--${(idx === -1 ? 0 : idx) + 1}`;
}

export function myDoneActions(session: Session): number {
  return session.progress.reduce((n, p) => n + Number(p.my_question) + Number(p.my_answer), 0);
}

/** Questions and answers written by everybody. */
export function allDoneActions(session: Session): number {
  return session.progress.reduce((n, p) => n + p.questions_written + p.answers_written, 0);
}

export type Badge = { label: string; tone: "turn" | "wait" | "done" };

/**
 * The step the session ends with if everybody agrees to finish early. It
 * mirrors Session.CurrentStep of the backend: the latest step anybody has
 * written on, or the first unfinished one if it is later.
 */
export function currentStepOf(session: Session): number {
  let touched = 0;
  session.progress.forEach((p, i) => {
    if (p.questions_written > 0) touched = i;
  });
  const unfinished = session.progress.findIndex((p) => p.answers_written < session.players.length);
  return unfinished === -1 ? touched : Math.max(touched, unfinished);
}

export function canFinishEarly(session: Session): boolean {
  return session.status === "in_work" && currentStepOf(session) + 1 < session.steps_amount;
}

export function badgeOf(session: Session, me: string): Badge {
  const stage = stageOf(session);
  const votes = session.finish_votes;
  if (canFinishEarly(session) && votes.length > 0 && !votes.includes(me)) {
    return { label: "Предлагают закончить", tone: "turn" };
  }
  switch (stage.kind) {
    case "lobby":
      return session.host === me
        ? { label: session.players.length > 1 ? "Можно начинать" : "Ждём игроков", tone: "turn" }
        : { label: "Ждём начала", tone: "wait" };
    case "done":
      return { label: "Можно разворачивать", tone: "done" };
    case "write":
    case "answer":
      return { label: "Ваш ход", tone: "turn" };
    default:
      return { label: "Ждём остальных", tone: "wait" };
  }
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
