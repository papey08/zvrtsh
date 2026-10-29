import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// In development the API is proxied to the backend started with `make run-back`.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      // ws: true proxies the WebSocket of /zvrtsh/api/v1/events as well;
      // the Host header is kept, the backend compares it with Origin
      "/zvrtsh/api": { target: "http://localhost:8080", ws: true },
    },
  },
});
