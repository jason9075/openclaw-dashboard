# OpenClaw Dashboard

A lightweight, single-binary monitoring tool for the OpenClaw Agent ecosystem. Built with Go and Vanilla web technologies (Solid Nord design system).

## Core Features

### 1. Event-Driven Real-time Updates
Instead of periodic polling, OpenClaw notifies the dashboard immediately when events occur (commands, starts, stops).
- **Backend**: Implements `POST /api/hooks/receive`.
- **Frontend**: Uses **Server-Sent Events (SSE)** via `GET /api/events`.
- **Safety Fallback**: 60s periodic poll.

### 2. Real-time Skill Usage Hints
Uses OS-level file monitoring (`fsnotify`) to watch OpenClaw transcript files (`*.jsonl`).
- **Immediate Feedback**: When an agent calls a skill, a "Toast" notification appears in the UI showing exactly which skill is being executed.
- **Support**: Works with both Claude-style `tool_use` and OpenAI-style `tool_calls`.

### 3. Multi-Agent Ecosystem Alignment
Designed to perfectly match OpenClaw's [Isolated Multi-Agent architecture](https://docs.openclaw.ai/concepts/multi-agent).
- **Agent Personas**: Visualizes long-lived agent "brains" (Manager, Researcher, Coder, etc.).
- **Dynamic Identity**: Automatically parses `IDENTITY.md` and `openclaw.json` for agent names and emojis.
- **Isolation aware**: Correct mapping of Workspace, Agent Directory, and Session folders.

### 4. Smart Skill Management
- **Skill Filtering**: Automatically distinguishes between **External Skills** (installed in `~/.openclaw/skills` or configured `extraDirs`) and built-in system tools.
- **Clean UI**: Only user-installed/managed skills are listed to reduce clutter.

### 5. Advanced System & Cost Monitoring
- **Gateway Health**: Monitors the `openclaw-gateway` process (PID, Uptime, Memory/RSS).
- **Cost Analysis**: Real-time parsing of session transcripts to calculate usage costs and token consumption across all models (GPT-5, Claude, Gemini, etc.).

---

## Configuration & Installation

### Automatic State Discovery
The dashboard intelligently looks for OpenClaw data in the following order:
1.  `OPENCLAW_STATE_DIR` or `CLAWDBOT_STATE_DIR` env variables.
2.  Local project `.openclaw` directory.
3.  `/home/jason9075/.openclaw` (Target system specific fallback).
4.  Standard `~/.openclaw` directory.

### Installing the Dashboard Hook into OpenClaw
To enable real-time notifications:

1.  **Copy the Hook**:
    ```bash
    cp -r ./hooks/dashboard-bridge ~/.openclaw/hooks/
    ```
2.  **Enable the Hook**:
    ```bash
    openclaw hooks enable dashboard-bridge
    ```
3.  **Set Dashboard URL (Optional)**:
    Default is `http://localhost:8080`. Set `OPENCLAW_DASHBOARD_URL` if different.
4.  **Restart OpenClaw Gateway**.

---

## Development Workflow: Remote Sync

We use a remote synchronization workflow to develop on a local machine and run/test on a remote target server (e.g., a robot or a dev server).

### 1. Host Machine Setup (Your Laptop)

#### SSH Configuration (Crucial for Speed)
To ensure `rsync` is fast and doesn't re-negotiate the connection on every file save, you **must** configure SSH multiplexing in your `~/.ssh/config`.

Add the following configuration (adjust `Host`, `Hostname`, `User`, and `IdentityFile` as needed):

```ssh
Host claw
    Hostname 10.90.0.197
    User clawbot
    IdentityFile ~/.ssh/id_ed25519
    IdentitiesOnly yes
    Port 22
    # Optimization for fast repeated connections
    ControlMaster auto
    ControlPath ~/.ssh/ansible-%r@%h:%p
    ControlPersist 10m
```

#### Start the Sync Script
Run the synchronization script in a separate terminal. This script watches for file changes and syncs them to the remote server immediately.

```bash
just sync
```

- **Features**: 
    - **Debounced**: Waits for 500ms after a file change to coalesce burst events (e.g., "Save All").
    - **Fast**: Uses `rsync` with a persistent SSH connection.
    - **Smart**: Only syncs relevant files (`.go`, `.html`, `.js`, etc.).

### 2. Target Machine Setup (Remote Server)

SSH into the remote machine and run the application. You can use `just air` for live reloading.

```bash
# SSH into the target
ssh claw

# Go to the project directory
cd ~/projects/openclaw-dashboard

# Run with hot reload (Recommended)
# If you have 'air' installed and configured (.air.toml exists)
just air

# OR run directly
just run
```

### Workflow Summary
1.  **Edit** code locally in your favorite IDE.
2.  **Save** the file.
3.  `just sync` automatically uploads the changes to the remote server.
4.  `just air` (on remote) detects the file change and recompiles/restarts the server.
5.  **Refresh** your browser to see changes.
