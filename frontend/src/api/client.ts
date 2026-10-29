import createClient, { type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Session = components["schemas"]["Session"];
export type StepProgress = components["schemas"]["StepProgress"];
export type Invite = components["schemas"]["Invite"];
export type Results = components["schemas"]["Results"];
export type EventsAuthMessage = components["schemas"]["EventsAuthMessage"];
export type EventsServerMessage = components["schemas"]["EventsServerMessage"];

// basePath must match the server url in api/openapi.yaml.
export const api = createClient<paths>({ baseUrl: "/zvrtsh/api/v1" });

let token: string | null = null;
let onUnauthorized: (() => void) | null = null;

export function setToken(value: string | null) {
  token = value;
}

/** Called when the server rejects the saved token. */
export function setUnauthorizedHandler(handler: (() => void) | null) {
  onUnauthorized = handler;
}

const auth: Middleware = {
  onRequest({ request }) {
    if (token) {
      request.headers.set("Authorization", `Bearer ${token}`);
    }
    return request;
  },
  onResponse({ request, response }) {
    if (response.status === 401 && request.headers.has("Authorization")) {
      onUnauthorized?.();
    }
    return response;
  },
};
api.use(auth);

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

/** Unwraps a result of the openapi-fetch client, throwing ApiError on failure. */
export async function call<T>(
  request: Promise<{ data?: T; error?: unknown; response: Response }>,
): Promise<T> {
  let result: { data?: T; error?: unknown; response: Response };
  try {
    result = await request;
  } catch {
    throw new ApiError(0, "network error");
  }
  if (!result.response.ok) {
    const error = result.error as { error?: string } | undefined;
    throw new ApiError(result.response.status, error?.error ?? "");
  }
  return result.data as T;
}

const messages: Record<string, string> = {
  "invalid credentials or token": "Неверное имя или пароль",
  "user with given name already exists": "Это имя уже занято",
  "invite code does not exist or has expired": "Код не найден или уже устарел",
  "there is no session with required id": "Такой партии нет — возможно, она устарела",
  "you are not a player of this session": "Это чужая партия",
  "step is not available right now": "Этот шаг пока недоступен",
  "session is not finished yet": "Партия ещё не закончена",
};

/** Turns any error into a message for the player. */
export function humanize(err: unknown): string {
  if (err instanceof ApiError) {
    if (err.status === 0 || err.status >= 502) {
      return "Сервер недоступен, попробуйте чуть позже";
    }
    if (err.message.includes("yourself")) {
      return "Нельзя играть с самим собой — отправьте код другу";
    }
    if (messages[err.message]) {
      return messages[err.message];
    }
    if (err.status === 400) {
      return "Проверьте введённые данные";
    }
  }
  return "Что-то пошло не так, попробуйте ещё раз";
}
