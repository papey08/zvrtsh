import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { setToken, setUnauthorizedHandler } from "../api/client";
import { useToast } from "./toast";

export type Auth = { name: string; token: string };

type AuthContextValue = {
  auth: Auth | null;
  login(auth: Auth): void;
  logout(): void;
};

const KEY = "zvrtsh.auth";

function loadAuth(): Auth | null {
  try {
    const raw = localStorage.getItem(KEY);
    const auth = raw ? (JSON.parse(raw) as Auth) : null;
    setToken(auth?.token ?? null);
    return auth;
  } catch {
    return null;
  }
}

function saveAuth(auth: Auth | null) {
  setToken(auth?.token ?? null);
  window.scrollTo({ top: 0 });
  try {
    if (auth) localStorage.setItem(KEY, JSON.stringify(auth));
    else localStorage.removeItem(KEY);
  } catch {
    // the player just has to log in again next time
  }
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState(loadAuth);
  const toast = useToast();

  const login = useCallback((value: Auth) => {
    saveAuth(value);
    setAuth(value);
  }, []);

  const logout = useCallback(() => {
    saveAuth(null);
    setAuth(null);
  }, []);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      logout();
      toast("Вход устарел — войдите снова", "error");
    });
    return () => setUnauthorizedHandler(null);
  }, [logout, toast]);

  const value = useMemo(() => ({ auth, login, logout }), [auth, login, logout]);
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used inside AuthProvider");
  return value;
}

/** Name of the logged in player; only for screens behind the login. */
export function useMe(): string {
  const { auth } = useAuth();
  if (!auth) throw new Error("useMe must be used after login");
  return auth.name;
}
