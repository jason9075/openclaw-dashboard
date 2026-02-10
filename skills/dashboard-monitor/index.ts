import { defineSkill } from "../../plugin-sdk/index.js";

export default defineSkill({
  id: "dashboard-monitor",
  name: "Dashboard Monitor",
  description: "Monitor OpenClaw performance and costs via Dashboard API",
  
  tools: {
    dashboard_get_status: {
      description: "Fetch real-time status from OpenClaw Dashboard",
      execute: async () => {
        try {
          const response = await fetch("http://localhost:8080/api/status");
          if (!response.ok) {
            return { error: `Failed to fetch dashboard: ${response.statusText}` };
          }
          return await response.json();
        } catch (err) {
          return { error: `Connection error: ${String(err)}` };
        }
      }
    }
  }
});
