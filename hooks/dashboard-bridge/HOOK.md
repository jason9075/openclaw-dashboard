---
name: dashboard-bridge
description: "Notifies OpenClaw Dashboard of agent events"
metadata:
  { "openclaw": { "emoji": "📊", "events": ["command", "gateway:startup"] } }
---

# Dashboard Bridge Hook

This hook sends real-time notifications to your OpenClaw Dashboard whenever an event occurs.

## Configuration

Ensure your dashboard is running at `http://localhost:8080` (default).
If it's running elsewhere, you can configure the endpoint in OpenClaw config.
