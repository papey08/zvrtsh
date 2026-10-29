import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource-variable/unbounded";
import "@fontsource-variable/manrope";
import "@fontsource-variable/caveat";
import "./styles.css";
import { App } from "./App";
import { AuthProvider } from "./lib/auth";
import { EventsProvider } from "./lib/events";
import { ToastProvider } from "./lib/toast";

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ToastProvider>
      <AuthProvider>
        <EventsProvider>
          <App />
        </EventsProvider>
      </AuthProvider>
    </ToastProvider>
  </StrictMode>,
);
