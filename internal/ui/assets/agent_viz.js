
document.addEventListener('DOMContentLoaded', () => {
    fetchAgents();
    setInterval(fetchAgents, 5000); // Poll every 5s

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

async function fetchAgents() {
    const listContainer = document.getElementById('full-agent-list');
    const sourceBadge = document.getElementById('data-source-indicator');

    try {
        const localResp = await fetch('/api/status');
        if (!localResp.ok) {
            throw new Error("Local status fetch failed");
        }
        const localData = await localResp.json();
        const agents = localData.sub_agents || [];
        
        if (sourceBadge) sourceBadge.textContent = "Local";
        renderList(agents);
    } catch (err) {
        console.error("Fetch failed", err);
        if (listContainer) {
            listContainer.innerHTML = '<div class="empty-state">Failed to load agents.</div>';
        }
    }
}

function renderList(agents) {
    const listContainer = document.getElementById('full-agent-list');
    if (!agents || agents.length === 0) {
        listContainer.innerHTML = '<div class="empty-state">No active agents found.</div>';
        return;
    }

    // Sort: Running first, then by name
    agents.sort((a, b) => {
        const statusScore = status => status.toLowerCase() === 'running' ? 2 : (status.toLowerCase() === 'failed' ? 1 : 0);
        const scoreA = statusScore(a.status || '');
        const scoreB = statusScore(b.status || '');
        if (scoreA !== scoreB) return scoreB - scoreA;
        return (a.name || '').localeCompare(b.name || '');
    });

    listContainer.innerHTML = agents.map(agent => {
        const status = (agent.status || 'Unknown').toLowerCase();
        // Handle potentially different data field names from remote
        const name = agent.name || agent.id || 'Unknown Agent';
        const duration = agent.duration || agent.uptime || '--';
        const tokens = agent.tokens || 0;
        const cost = agent.cost || 0;

        return `
        <div class="agent-card-full status-${status}">
            <div class="agent-info">
                <h3>${name}</h3>
                <div class="agent-meta">
                    <span>⏱ ${duration}</span>
                    <span>🧠 ${tokens.toLocaleString()} tkns</span>
                    <span>💰 $${typeof cost === 'number' ? cost.toFixed(4) : cost}</span>
                </div>
            </div>
            <div class="agent-actions">
                 <span class="badge-lg badge ${status}">${status}</span>
            </div>
        </div>
        `;
    }).join('');
}
