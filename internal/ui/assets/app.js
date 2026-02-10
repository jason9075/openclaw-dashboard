document.addEventListener('DOMContentLoaded', () => {
    fetchStatus();
    setInterval(fetchStatus, 60000); // 60 seconds
    startCountdown(60);

    // Slide Menu Logic
    const menu = document.getElementById('slide-menu');
    const overlay = document.getElementById('overlay');
    const openBtn = document.getElementById('menu-toggle-btn');
    const closeBtn = document.getElementById('close-menu-btn');

    function openMenu() {
        menu.classList.add('open');
        overlay.classList.add('active');
    }

    function closeMenu() {
        menu.classList.remove('open');
        overlay.classList.remove('active');
    }

    openBtn.addEventListener('click', openMenu);
    closeBtn.addEventListener('click', closeMenu);
    overlay.addEventListener('click', closeMenu);
});

let countdownTimer;

function startCountdown(seconds) {
    let counter = seconds;
    const el = document.getElementById('countdown');
    
    clearInterval(countdownTimer);
    countdownTimer = setInterval(() => {
        counter--;
        if (counter < 0) counter = seconds;
        el.textContent = counter;
    }, 1000);
}

async function fetchStatus() {
    try {
        const response = await fetch('/api/status');
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }
        const data = await response.json();
        updateDashboard(data);
        
        // Reset countdown on successful fetch
        startCountdown(60);
    } catch (error) {
        console.error('Error fetching status:', error);
        const statusEl = document.getElementById('status-indicator');
        if (statusEl) {
            statusEl.textContent = 'Offline';
            statusEl.className = 'status-offline';
        }
    }
}

function updateDashboard(data) {
    // Global Status
    const statusEl = document.getElementById('status-indicator');
    statusEl.textContent = 'Online';
    statusEl.className = 'status-online';

    // Footer Info
    if (data.uptime) document.getElementById('uptime-display').textContent = data.uptime;
    if (data.system && data.system.compaction_mode) {
        document.getElementById('compaction-mode').textContent = data.system.compaction_mode;
    }

    // Alerts Banner
    const alertsBanner = document.getElementById('alerts-banner');
    if (data.alerts && data.alerts.length > 0) {
        const critical = data.alerts.filter(a => a.level === 'ERROR' || a.level === 'CRITICAL');
        if (critical.length > 0) {
           alertsBanner.innerHTML = critical.map(a => `
               <div class="warning-banner">
                   <span>⚠️ ${a.message}</span>
                   <span>${a.time}</span>
               </div>
           `).join('');
        } else {
            alertsBanner.innerHTML = '';
        }
    } else {
        alertsBanner.innerHTML = '';
    }

    // System Health
    const healthContent = document.getElementById('sys-health-content');
    if (data.system) {
        healthContent.innerHTML = `
            <div class="stat-row"><strong>Hostname:</strong> <span>${data.system.hostname}</span></div>
            <div class="stat-row"><strong>Kernel:</strong> <span>${data.system.kernel}</span></div>
            <div class="stat-row"><strong>Load Avg:</strong> <span>${data.system.load_average}</span></div>
            <div class="stat-row"><strong>Memory:</strong> <span>${data.system.memory_usage}</span></div>
            <div class="stat-row"><strong>Disk (/):</strong> <span>${data.system.disk_usage}</span></div>
        `;
    }

    // Costs
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

    // Token Usage
    renderTokenUsage(data.token_usage);

    // Sub-Agents
    const subAgentsContent = document.getElementById('subagents-content');
    if (data.sub_agents && data.sub_agents.length > 0) {
        subAgentsContent.innerHTML = data.sub_agents.map(sa => `
            <div class="subagent-item sa-status-${sa.status.toLowerCase()}">
                <div>
                    <div><strong>${sa.name}</strong></div>
                    <small>${sa.duration} | ${sa.tokens} tokens</small>
                </div>
                <div>$${sa.cost.toFixed(4)}</div>
            </div>
        `).join('');
    } else {
        subAgentsContent.innerHTML = '<div class="empty-state">No sub-agent activity</div>';
    }

    // Kanban
    // (Reuse existing logic or update if needed)
    const kanbanContent = document.getElementById('kanban-content');
     if (data.todos && data.todos.length > 0) {
        // Group by status if we want a real board, but for now list is fine or grouped list
        // Let's just list with status badges
        kanbanContent.innerHTML = data.todos.map(todo => `
            <div class="todo-item priority-${todo.priority ? todo.priority.toLowerCase() : 'low'}">
                <span class="todo-status">[${todo.status}]</span> ${todo.title}
            </div>
        `).join('');
    } else {
        kanbanContent.innerHTML = '<div class="empty-state">No tasks</div>';
    }

    // Active Sessions
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

    // Cron Jobs
    const cronContent = document.getElementById('cron-content');
    if (data.cron_jobs && data.cron_jobs.length > 0) {
        cronContent.innerHTML = `
            <table class="data-table">
                <thead><tr>width="20%">Schedule</th><th>Command</th></tr></thead>
                <tbody>
                    ${data.cron_jobs.map(j => `<tr><td><code>${j.schedule}</code></td><td><code>${j.command}</code></td></tr>`).join('')}
                </tbody>
            </table>
        `;
    } else {
        cronContent.innerHTML = '<div class="empty-state">No cron jobs</div>';
    }

    // Models
    const modelsContent = document.getElementById('models-content');
    if (data.models && data.models.length > 0) {
        modelsContent.innerHTML = `
            <ul style="list-style: none; padding: 0;">
                ${data.models.map(m => `<li style="padding: 5px 0; border-bottom: 1px solid var(--nord2);"><strong>${m.name}</strong> <small>(${m.type})</small></li>`).join('')}
            </ul>
        `;
    } else {
        modelsContent.innerHTML = '<div class="empty-state">No models avail</div>';
    }

    // Skills
    const skillsContent = document.getElementById('skills-content');
    if (data.skills && data.skills.length > 0) {
        skillsContent.innerHTML = `
            <ul style="list-style: none; padding: 0;">
                ${data.skills.map(s => `<li style="padding: 5px 0; border-bottom: 1px solid var(--nord2);"><strong>${s.name}</strong></li>`).join('')}
            </ul>
        `;
    } else {
        skillsContent.innerHTML = '<div class="empty-state">No skills loaded</div>';
    }

    // Git Log
    const gitContent = document.getElementById('git-content');
    if (data.git_log && data.git_log.length > 0) {
         gitContent.innerHTML = data.git_log.map(commit => `
            <div style="padding: 5px 0; border-bottom: 1px solid var(--nord2); font-family: monospace; font-size: 0.9em;">
                <span style="color: var(--nord10); margin-right: 5px;">${commit.hash}</span>
                <span style="color: var(--nord4);">${commit.message}</span>
            </div>
        `).join('');
    } else {
        gitContent.innerHTML = '<div class="empty-state">No git log</div>';
    }
}

// Token View Toggle
let currentTokenView = 'today'; // 'today' or 'all-time'
let cachedTokenData = [];

function toggleTokenView(view) {
    currentTokenView = view;
    document.querySelectorAll('.toggle-btn').forEach(btn => btn.classList.remove('active'));
    // Find the button with correct onclick handler text - logic simplified for demo
    // In real app, bind ID or usage
    if (view === 'today') document.querySelector('button[onclick="toggleTokenView(\'today\')"]').classList.add('active');
    else document.querySelector('button[onclick="toggleTokenView(\'all-time\')"]').classList.add('active');

    renderTokenUsage(cachedTokenData);
}

function renderTokenUsage(data) {
    cachedTokenData = data || [];
    const container = document.getElementById('token-usage-content');
    
    if (!cachedTokenData || cachedTokenData.length === 0) {
        container.innerHTML = '<div class="empty-state">No token data</div>';
        return;
    }

    // Mock logic for today vs all-time since backend structure only has totals
    // In a real app, TokenStats would have Today/AllTime fields.
    // We'll simulate it by showing logic based on view
    
    container.innerHTML = cachedTokenData.map(stat => {
        // Mock calculation for demo purposes
        const displayTokens = currentTokenView === 'today' ? Math.round(stat.input_tokens * 0.1) : stat.input_tokens;
        const displayCost = currentTokenView === 'today' ? stat.total_cost * 0.1 : stat.total_cost;
        const maxTokens = 100000; // Arbitrary max for bar
        const percent = Math.min((displayTokens / maxTokens) * 100, 100);

        return `
            <div class="token-bar-row">
                <div style="display:flex; justify-content:space-between; margin-bottom:2px;">
                    <span style="font-size:0.9rem;">${stat.model}</span>
                    <span style="font-size:0.8rem; color:var(--nord4);">${displayTokens.toLocaleString()} tkns</span>
                </div>
                <div class="token-progress-bg" style="height:6px; background:var(--nord0); border-radius:3px; overflow:hidden;">
                    <div class="token-progress" style="width:${percent}%; background:var(--nord8); height:100%;"></div>
                </div>
                <div style="text-align:right; font-size:0.8rem; color:var(--nord9); margin-top:2px;">$${displayCost.toFixed(4)}</div>
            </div>
            <div style="margin-bottom: 8px;"></div>
        `;
    }).join('');
}
