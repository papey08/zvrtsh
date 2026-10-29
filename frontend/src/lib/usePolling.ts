import { useEffect, useRef } from "react";

/**
 * Calls fn right away and then every intervalMs while enabled. Polling pauses
 * in a hidden tab and resumes immediately when the tab becomes visible.
 */
export function usePolling(fn: () => Promise<unknown>, intervalMs: number, enabled = true) {
  const saved = useRef(fn);
  useEffect(() => {
    saved.current = fn;
  }, [fn]);

  useEffect(() => {
    if (!enabled) return;

    let stopped = false;
    let inFlight = false;
    let timer: number | undefined;

    const tick = async () => {
      if (stopped || inFlight) return;
      window.clearTimeout(timer);
      if (!document.hidden) {
        inFlight = true;
        try {
          await saved.current();
        } catch {
          // errors are shown by the callback itself, keep polling
        } finally {
          inFlight = false;
        }
      }
      if (!stopped) timer = window.setTimeout(tick, intervalMs);
    };

    const onVisibility = () => {
      if (!document.hidden) void tick();
    };

    void tick();
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      stopped = true;
      window.clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [intervalMs, enabled]);
}
