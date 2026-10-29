import { useEffect, useState } from "react";

export type Route = { name: "lobby" } | { name: "game"; id: string } | { name: "join"; code: string };

export const links = {
  lobby: "#/",
  game: (id: string) => `#/game/${encodeURIComponent(id)}`,
  join: (code: string) => `#/join/${encodeURIComponent(code)}`,
};

export function parseRoute(hash: string): Route {
  const [name, param] = hash
    .replace(/^#\/?/, "")
    .split("/")
    .filter(Boolean)
    .map((part) => {
      try {
        return decodeURIComponent(part);
      } catch {
        return part;
      }
    });
  if (name === "game" && param) return { name: "game", id: param };
  if (name === "join" && param) return { name: "join", code: param };
  return { name: "lobby" };
}

/**
 * Takes the invite code from what the player has pasted: the code itself or
 * the invite link (…#/join/<code>) from "Скопировать ссылку".
 */
export function inviteCodeFrom(input: string): string {
  const text = input.trim();
  const hash = text.indexOf("#");
  if (hash !== -1) {
    const route = parseRoute(text.slice(hash));
    if (route.name === "join") return route.code.trim();
  }
  return text;
}

export function navigate(link: string) {
  window.location.hash = link;
}

export function useRoute(): Route {
  const [route, setRoute] = useState(() => parseRoute(window.location.hash));
  useEffect(() => {
    const onChange = () => {
      setRoute(parseRoute(window.location.hash));
      window.scrollTo({ top: 0 });
    };
    window.addEventListener("hashchange", onChange);
    return () => window.removeEventListener("hashchange", onChange);
  }, []);
  return route;
}
