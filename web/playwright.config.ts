import { defineConfig } from "@playwright/test";
import path from "node:path";
const binary = path.resolve(
  "../bin",
  process.platform === "win32" ? "reconcile.exe" : "reconcile",
);
export default defineConfig({
  testDir: "tests",
  workers: 1,
  use: {
    baseURL: "http://127.0.0.1:8082",
    channel: process.platform === "win32" ? "msedge" : undefined,
    headless: true,
  },
  webServer: {
    command: `"${binary}" -addr 127.0.0.1:8082 -db ../data/browser-test-${Date.now()}.db -web dist`,
    url: "http://127.0.0.1:8082",
    reuseExistingServer: false,
  },
});
