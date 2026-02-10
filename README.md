# OpenClaw Dashboard

This is the dashboard for the OpenClaw project.

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
