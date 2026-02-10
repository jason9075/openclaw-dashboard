
document.addEventListener('DOMContentLoaded', () => {
    fetchAgents();
    setInterval(fetchAgents, 60000); // Fallback poll every 60s

    // Real-time updates via SSE
    setupSSE();

    // Slide Menu Logic (Reused)
    const menu = document.getElementById('slide-menu');
    const overlay = document.getElementById('overlay');
    const openBtn = document.getElementById('menu-toggle-btn');
    const closeBtn = document.getElementById('close-menu-btn');
    const refreshBtn = document.getElementById('refresh-agents-btn');

    if (openBtn) openBtn.addEventListener('click', () => { menu.classList.add('open'); overlay.classList.add('active'); });
    if (closeBtn) closeBtn.addEventListener('click', () => { menu.classList.remove('open'); overlay.classList.remove('active'); });
    if (overlay) overlay.addEventListener('click', () => { menu.classList.remove('open'); overlay.classList.remove('active'); });
    if (refreshBtn) refreshBtn.addEventListener('click', fetchAgents);
});

function setupSSE() {
    const eventSource = new EventSource('/api/events');
    
    eventSource.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            if (data.type === 'refresh') {
                console.log('Real-time refresh triggered by hook:', data.payload);
                fetchAgents();
            } else if (data.type === 'skill_use') {
                showSkillHint(data.payload);
            }
        } catch (err) {
            console.error('Failed to parse SSE event:', err);
        }
    };

    eventSource.onerror = (err) => {
        console.warn('SSE connection lost, retrying...', err);
    };
}

function showSkillHint(payload) {
    const { agent_id, skill } = payload;
    
    // Find the persona card or agent card
    const personaCard = document.querySelector(`.agent-persona-card[data-id="${agent_id}"]`) || 
                        document.querySelector(`.agent-card-full h3:contains("${agent_id}")`)?.closest('.agent-card-full');
    
    // For now, simpler: show a toast notification
    const toast = document.createElement('div');
    toast.className = 'skill-toast';
    toast.innerHTML = `
        <div class="toast-content">
            <span class="toast-icon">⚡</span>
            <div class="toast-info">
                <strong>${agent_id}</strong> is using skill:
                <code>${skill}</code>
            </div>
        </div>
    `;
    document.body.appendChild(toast);
    
    setTimeout(() => {
        toast.classList.add('fade-out');
        setTimeout(() => toast.remove(), 500);
    }, 3000);
}

async function fetchAgents() {
    const listContainer = document.getElementById('full-agent-list');
    const sourceBadge = document.getElementById('data-source-indicator');
    const statusIndicator = document.getElementById('status-indicator');

    try {
        const localResp = await fetch('/api/status');
        if (!localResp.ok) {
            throw new Error("Local status fetch failed");
        }
        const localData = await localResp.json();
        
        if (sourceBadge) sourceBadge.textContent = "Local";
        
        if (statusIndicator) {
            const gwStatus = (localData.system && localData.system.gateway_status) || 'offline';
            const statusText = gwStatus === 'online' ? 'Online' : 'Gateway Offline';
            statusIndicator.textContent = statusText;
            statusIndicator.className = gwStatus === 'online' ? 'status-online' : 'status-offline';
        }

        renderAgents(localData);
    } catch (err) {
        console.error("Fetch failed", err);
        if (statusIndicator) {
            statusIndicator.textContent = "Error";
            statusIndicator.className = "status-offline";
        }
        if (listContainer) {
            listContainer.innerHTML = '<div class="empty-state">Failed to load agents.</div>';
        }
    }
}

function renderAgents(data) {
    const listContainer = document.getElementById('full-agent-list');
    const personas = data.personas || [];
    const runs = data.sub_agents || [];

    if (personas.length === 0 && runs.length === 0) {
        listContainer.innerHTML = '<div class="empty-state">No agents or activity found.</div>';
        return;
    }

    let html = '';

    // 1. Render Personas (Isolated Brains as per Docs)
    if (personas.length > 0) {
        html += '<h2 class="section-title">Agent Personas (Isolated Brains)</h2>';
        html += personas.map(p => {
            return `
            <div class="agent-persona-card ${p.is_default ? 'default-agent' : ''}" data-id="${p.id}">
                <div class="persona-header">
                    <span class="persona-icon">${p.emoji || '🧠'}</span>
                    <div class="persona-info">
                        <h3>${p.name} ${p.is_default ? '<small>(Default)</small>' : ''}</h3>
                        <code>ID: ${p.id}</code>
                    </div>
                    <span class="badge online">Active</span>
                </div>
                <div class="persona-paths">
                    <div class="path-item">
                        <span class="label">Workspace:</span>
                        <code class="path">${p.workspace}</code>
                    </div>
                    <div class="path-item">
                        <span class="label">Agent Dir:</span>
                        <code class="path">${p.agent_dir}</code>
                    </div>
                </div>
                ${p.skills && p.skills.length > 0 ? `
                <div class="persona-skills">
                    <span class="label">Enabled Skills:</span>
                    <div class="skill-tags">
                        ${p.skills.map(s => `<span class="skill-tag">${s}</span>`).join('')}
                    </div>
                </div>
                ` : ''}
            </div>
            `;
        }).join('');
    }

    // 2. Render Recent Task Activity
    if (runs.length > 0) {
        html += '<h2 class="section-title" style="margin-top: 40px;">Recent Task Activity (Sub-agents)</h2>';
        
        // Sort runs: Running first
        runs.sort((a, b) => {
            const statusScore = status => status.toLowerCase() === 'running' ? 2 : (status.toLowerCase() === 'failed' ? 1 : 0);
            return statusScore(b.status || '') - statusScore(a.status || '');
        });

        html += runs.map(agent => {
            const status = (agent.status || 'Unknown').toLowerCase();
            const name = agent.name || agent.id || 'Unknown Agent';
            const duration = agent.duration || agent.uptime || '--';
            const costVal = parseFloat(agent.cost);
            const costStr = isNaN(costVal) ? '0.0000' : costVal.toFixed(4);
            const tokenStr = (parseInt(agent.tokens) || 0).toLocaleString();

            return `
            <div class="agent-card-full status-${status}">
                <div class="agent-info">
                    <h3>${name}</h3>
                    <div class="agent-meta">
                        <span>⏱ ${duration}</span>
                        <span>🧠 ${tokenStr} tkns</span>
                        <span>💰 $${costStr}</span>
                    </div>
                </div>
                <div class="agent-actions">
                     <span class="badge-lg badge ${status}">${status}</span>
                </div>
            </div>
            `;
        }).join('');
    }

    listContainer.innerHTML = html;
}
