import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { EventsAuthMessage, EventsServerMessage, Session } from "../api/client";
import { useAuth } from "./auth";
import { useToast } from "./toast";

export type ConnectionStatus = "connecting" | "online" | "offline";

/** Gets an updated session, or only the id of a deleted one. */
type Listener = (session: Session | null, deletedId?: string) => void;

type EventsContextValue = {
  status: ConnectionStatus;
  /** Grows every time the socket becomes ready: the moment to reload the state. */
  syncKey: number;
  subscribe(listener: Listener): () => void;
};

const EventsContext = createContext<EventsContextValue | null>(null);

// close codes from the description of /events in api/openapi.yaml
const CLOSE_UNAUTHORIZED = 1008;

const MAX_RETRY_MS = 15000;

function eventsUrl(): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/zvrtsh/api/v1/events`;
}

/** Keeps one WebSocket per logged in player and reconnects it when it drops. */
export function EventsProvider({ children }: { children: ReactNode }) {
  const { auth, logout } = useAuth();
  const toast = useToast();
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const [syncKey, setSyncKey] = useState(0);
  const listeners = useRef(new Set<Listener>());
  const token = auth?.token;

  useEffect(() => {
    if (!token) return;

    let socket: WebSocket | null = null;
    let stopped = false;
    let attempt = 0;
    let timer: number | undefined;

    const connect = () => {
      window.clearTimeout(timer);
      setStatus("connecting");
      const ws = new WebSocket(eventsUrl());
      socket = ws;

      ws.onopen = () => {
        const auth: EventsAuthMessage = { type: "auth", token };
        ws.send(JSON.stringify(auth));
      };

      ws.onmessage = (e) => {
        const msg = JSON.parse(e.data as string) as EventsServerMessage;
        if (msg.type === "ready") {
          attempt = 0;
          setStatus("online");
          setSyncKey((k) => k + 1);
        } else if (msg.type === "session_updated" && msg.session) {
          for (const listener of listeners.current) listener(msg.session);
        } else if (msg.type === "session_deleted" && msg.session_id) {
          for (const listener of listeners.current) listener(null, msg.session_id);
        }
      };

      ws.onclose = (e) => {
        if (socket !== ws) return;
        socket = null;
        if (stopped) return;
        if (e.code === CLOSE_UNAUTHORIZED) {
          logout();
          toast("Вход устарел — войдите снова", "error");
          return;
        }
        setStatus("offline");
        // exponential backoff with jitter, so clients do not reconnect all at once
        const delay = Math.min(MAX_RETRY_MS, 1000 * 2 ** attempt++) * (0.75 + Math.random() / 2);
        timer = window.setTimeout(connect, delay);
      };
    };

    // reconnect at once when the network or the tab comes back
    const reconnectNow = () => {
      if (!socket && !stopped && !document.hidden) {
        attempt = 0;
        connect();
      }
    };

    connect();
    window.addEventListener("online", reconnectNow);
    document.addEventListener("visibilitychange", reconnectNow);
    return () => {
      stopped = true;
      window.clearTimeout(timer);
      window.removeEventListener("online", reconnectNow);
      document.removeEventListener("visibilitychange", reconnectNow);
      socket?.close(1000);
    };
  }, [token, logout, toast]);

  const subscribe = useCallback((listener: Listener) => {
    listeners.current.add(listener);
    return () => {
      listeners.current.delete(listener);
    };
  }, []);

  const value = useMemo(() => ({ status, syncKey, subscribe }), [status, syncKey, subscribe]);
  return <EventsContext.Provider value={value}>{children}</EventsContext.Provider>;
}

function useEvents(): EventsContextValue {
  const value = useContext(EventsContext);
  if (!value) throw new Error("useEvents must be used inside EventsProvider");
  return value;
}

export function useConnection(): { status: ConnectionStatus; syncKey: number } {
  const { status, syncKey } = useEvents();
  return { status, syncKey };
}

/**
 * Calls onUpdate with every updated session of the player and onDelete with
 * the id of every session cancelled by its host.
 */
export function useSessionEvents(onUpdate: (session: Session) => void, onDelete?: (id: string) => void) {
  const { subscribe } = useEvents();
  const saved = useRef({ onUpdate, onDelete });
  useEffect(() => {
    saved.current = { onUpdate, onDelete };
  }, [onUpdate, onDelete]);
  useEffect(
    () =>
      subscribe((session, deletedId) => {
        if (session) saved.current.onUpdate(session);
        else if (deletedId) saved.current.onDelete?.(deletedId);
      }),
    [subscribe],
  );
}

/** The newest of two snapshots of the same session. */
export function newer(current: Session | null, next: Session): Session {
  return current && current.version > next.version ? current : next;
}
