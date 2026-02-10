document.addEventListener('DOMContentLoaded', () => {
    fetchStatus();
    setInterval(fetchStatus, 60000); // 60 seconds
});

async function fetchStatus() {
    try {
        const response = await fetch('/api/status');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        updateDashboard(data);
    } catch (error) {
        console.error('Error fetching status:', error);
        document.getElementById('status-indicator').textContent = 'Offline';
        document.getElementById('status-indicator').style.backgroundColor = 'var(--nord11)';
    }
}

function updateDashboard(data) {
    // Update global status
    const statusEl = document.getElementById('status-indicator');
    statusEl.textContent = 'Online';
    statusEl.style.backgroundColor = 'var(--nord14)';

    // Update Uptime
    if (data.uptime) {
        document.getElementById('uptime-display').textContent = data.uptime;
    }

    // Update Costs
    const costsContent = document.getElementById('costs-content');
    if (data.costs && data.costs.length > 0) {
        costsContent.innerHTML = data.costs.map(cost => `
            <div class="cost-card">
                <span class="cost-label">${cost.label}</span>
                <span class="cost-value">${cost.unit}${cost.value.toFixed(2)}</span>
            </div>
        `).join('');
    } else {
        costsContent.innerHTML = '<div class="empty-state">No cost data</div>';
    }

    // Update Health Panel
    const healthContent = document.getElementById('sys-health-content');
    if (data.system) {
        healthContent.innerHTML = `
            <div class="stat-row"><strong>Load Avg:</strong> ${data.system.load_average}</div>
            <div class="stat-row"><strong>Memory:</strong> ${data.system.memory_usage}</div>
            <div class="stat-row"><strong>Disk (/):</strong> ${data.system.disk_usage}</div>
        `;
    }

    // Update Info Panel
    const infoContent = document.getElementById('sys-info-content');
    if (data.system) {
        infoContent.innerHTML = `
            <div class="stat-row"><strong>Hostname:</strong> ${data.system.hostname}</div>
            <div class="stat-row"><strong>Kernel:</strong> ${data.system.kernel}</div>
            <div class="stat-row"><strong>Uptime:</strong> ${data.system.uptime}</div>
        `;
    }

    // Update Alerts
    const alertsContent = document.getElementById('alerts-content');
    if (data.alerts && data.alerts.length > 0) {
        alertsContent.innerHTML = data.alerts.map(alert => `
            <div class="alert-item alert-${alert.level.toLowerCase()}">
                <span class="alert-time">[${alert.time}]</span>
                <span class="alert-msg">${alert.message}</span>
            </div>
        `).join('');
    } else {
        alertsContent.innerHTML = '<div class="empty-state">No active alerts</div>';
    }

    // Update Sessions
    const sessionsContent = document.getElementById('sessions-content');
    const sessions = data.sessions ? Object.values(data.sessions) : [];
    if (sessions.length > 0) {
        sessionsContent.innerHTML = `
            <table class="data-table">
                <thead><tr><th>User</th><th>IP</th><th>Started</th></tr></thead>
                <tbody>
                    ${sessions.map(s => `<tr><td>${s.user}</td><td>${s.remote_ip}</td><td>${s.started_at}</td></tr>`).join('')}
                </tbody>
            </table>
        `;
    } else {
        sessionsContent.innerHTML = '<div class="empty-state">No active sessions</div>';
    }

    // Update Kanban
    const kanbanContent = document.getElementById('kanban-content');
    if (data.todos && data.todos.length > 0) {
        kanbanContent.innerHTML = data.todos.map(todo => `
            <div class="todo-item priority-${todo.priority.toLowerCase()}">
                <span class="todo-status">[${todo.status}]</span> ${todo.title}
            </div>
        `).join('');
    } else {
        kanbanContent.innerHTML = '<div class="empty-state">No tasks pending</div>';
    }

    // Update Git Log
    const gitContent = document.getElementById('git-content');
    if (data.git_log && data.git_log.length > 0) {
        gitContent.innerHTML = data.git_log.map(commit => `
            <div class="git-commit">
                <span class="commit-hash">${commit.hash}</span>
                <span class="commit-msg">${commit.message}</span>
            </div>
        `).join('');
    } else {
        gitContent.innerHTML = '<div class="empty-state">No git history</div>';
    }

    // Update Cron Jobs
    const cronContent = document.getElementById('cron-content');
    if (data.cron_jobs && data.cron_jobs.length > 0) {
        cronContent.innerHTML = `
            <table class="data-table">
                <thead><tr><th>Schedule</th><th>Command</th></tr></thead>
                <tbody>
                    ${data.cron_jobs.map(j => `<tr><td><code>${j.schedule}</code></td><td><code>${j.command}</code></td></tr>`).join('')}
                </tbody>
            </table>
        `;
    } else {
        cronContent.innerHTML = '<div class="empty-state">No cron jobs configured</div>';
    }
}
