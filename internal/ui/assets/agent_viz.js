
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

    // Try Remote First
    let agents = [];
    let source = "Local";

    try {
        // Attempt to fetch from the remote dashboard API
        // We use 'cors' mode if possible, or hope for same-origin via proxy
        // Since it's a different port/IP, this depends on the server's CORS policy.
        // If the user is authenticated in the browser for that IP, credentials might work.
        const remoteUrl = 'http://10.90.0.197:18789/api/agents';
        const response = await fetch(remoteUrl, {
            method: 'GET',
            headers: { 'Accept': 'application/json' }
        });

        if (response.ok) {
            const data = await response.json();
            // Data format might differ. Assuming standard OpenClaw format or similar to ours.
            // If it returns a list directly or { agents: [...] }
            agents = Array.isArray(data) ? data : (data.agents || []);
            source = "Remote (10.90.0.197)";
        } else {
            throw new Error("Remote fetch failed");
        }
    } catch (e) {
        console.warn("Could not fetch from remote, falling back to local", e);
        // Fallback to local
        try {
            const localResp = await fetch('/api/status');
            const localData = await localResp.json();
            agents = localData.sub_agents || [];
            source = "Local (Fallback)";
        } catch (localErr) {
            console.error("Local fetch failed", localErr);
            listContainer.innerHTML = '<div class="empty-state">Failed to load agents.</div>';
            return;
        }
    }

    if (sourceBadge) sourceBadge.textContent = source;
    renderList(agents);
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
