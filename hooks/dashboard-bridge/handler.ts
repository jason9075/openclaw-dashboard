import type { HookHandler } from "../../src/hooks/hooks.js";

const handler: HookHandler = async (event) => {
  // Extract dashboard URL from env or default
  const dashboardUrl = process.env.OPENCLAW_DASHBOARD_URL || "http://localhost:8080";
  const target = `${dashboardUrl}/api/hooks/receive`;

  try {
    // Fire and forget (don't block agent)
    void fetch(target, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        event: event.type,
        action: event.action,
        sessionKey: event.sessionKey,
        timestamp: event.timestamp,
        source: event.context?.commandSource
      })
    }).catch(err => {
        // Silent fail if dashboard is down
    });
  } catch (err) {
    // ignore
  }
};

export default handler;
