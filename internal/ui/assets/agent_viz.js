
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
            console.log('SSE Event received:', data.type, data.payload);
            if (data.type === 'refresh') {
                fetchAgents();
                // If modal is open for this agent, reload it too
                const modal = document.getElementById('detail-modal');
                if (modal && modal.style.display === 'flex') {
                    const currentAgentId = modal.dataset.agentId;
                    const sessionSelect = document.getElementById('modal-session-select');
                    // Only auto-reload if we are on the LATEST session
                    if (sessionSelect && sessionSelect.selectedIndex === 0) {
                        loadSessionDetails(currentAgentId, sessionSelect.value, true);
                    }
                }
            } else if (data.type === 'skill_use') {
                showSkillHint(data.payload);
            } else if (data.type === 'agent_status') {
                updateAgentUIStatus(data.payload);
            } else if (data.type === 'reasoning_delta') {
                updateLiveReasoning(data.payload);
            }
        } catch (err) {
            console.error('Failed to parse SSE event:', err);
        }
    };

    eventSource.onerror = (err) => {
        console.warn('SSE connection lost, retrying...', err);
    };
}

function updateAgentUIStatus(payload) {
    const { agent_id, status } = payload;
    
    // If status changed significantly, we might want to re-fetch/re-render the whole list
    // to ensure sorting is correct (Thinking at top).
    // For now, let's just refresh the whole list to keep it simple and accurate.
    fetchAgents();
}

function updateLiveReasoning(payload) {
    const { agent_id, delta } = payload;
    const modal = document.getElementById('detail-modal');
    if (modal && modal.style.display === 'flex' && modal.dataset.agentId === agent_id) {
        const reasoningEl = document.getElementById('modal-reasoning');
        if (reasoningEl) {
            reasoningEl.textContent += delta;
            reasoningEl.scrollTop = reasoningEl.scrollHeight;
        }
    }
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

    // Sort Personas: Thinking first, then by LastActive (descending), then by ID
    personas.sort((a, b) => {
        if (a.status === 'thinking' && b.status !== 'thinking') return -1;
        if (a.status !== 'thinking' && b.status === 'thinking') return 1;
        
        // Both same status (e.g. both idle), sort by LastActive
        if (a.last_active !== b.last_active) {
            return b.last_active - a.last_active;
        }
        
        return a.id.localeCompare(b.id);
    });

    let html = '';

    // 1. Render Personas (Isolated Brains as per Docs)
    if (personas.length > 0) {
        html += '<h2 class="section-title">Agent Personas (Isolated Brains)</h2>';
        html += personas.map(p => {
            const isThinking = p.status === 'thinking';
            const isIdle = p.status === 'idle';
            return `
            <div class="agent-persona-card ${p.is_default ? 'default-agent' : ''} ${isThinking ? 'is-thinking' : ''} ${isIdle ? 'is-idle' : ''}" data-id="${p.id}">
                <div class="persona-header">
                    <span class="persona-icon">${p.emoji || '🧠'}</span>
                    <div class="persona-info">
                        <h3>${p.name} ${p.is_default ? '<small>(Default)</small>' : ''}</h3>
                        <code>ID: ${p.id}</code>
                    </div>
                    <div class="header-right">
                        <button class="btn-detail" onclick="openDetailModal('${p.id}')">Detail</button>
                        <span class="badge ${isThinking ? 'thinking' : 'online'}">${isThinking ? 'Thinking' : (isIdle ? 'Idle' : 'Active')}</span>
                    </div>
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

async function openDetailModal(agentId) {
    const modal = document.getElementById('detail-modal');
    const select = document.getElementById('modal-session-select');
    const conversationEl = document.getElementById('modal-conversation');
    
    if (!modal) return;

    modal.dataset.agentId = agentId;
    document.getElementById('modal-agent-name').textContent = agentId;
    conversationEl.innerHTML = '<div class="loading">Loading sessions...</div>';
    select.innerHTML = '';
    
    modal.style.display = 'flex';

    try {
        // 1. Fetch session list
        const listResp = await fetch(`/api/session/list?agentId=${agentId}`);
        if (!listResp.ok) throw new Error("Failed to load session list");
        const sessions = await listResp.json();
        
        if (!sessions || sessions.length === 0) {
            conversationEl.innerHTML = '<div class="empty-state">No sessions found for this agent.</div>';
            return;
        }

        // Sort: Newest first
        sessions.sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));

        sessions.forEach((s, idx) => {
            const opt = document.createElement('option');
            opt.value = s.id;
            const date = new Date(s.updated_at).toLocaleString();
            opt.textContent = `${s.id.substring(0, 8)}... (${date})`;
            if (idx === 0) opt.selected = true;
            select.appendChild(opt);
        });

        select.onchange = () => loadSessionDetails(agentId, select.value, false);

        // 2. Load latest session details
        await loadSessionDetails(agentId, sessions[0].id, false);

    } catch (err) {
        console.error(err);
        conversationEl.innerHTML = `<div class="empty-state">No sessions found or error loading data.</div>`;
    }
}

async function loadSessionDetails(agentId, sessionId, isAutoRefresh = false) {
    const conversationEl = document.getElementById('modal-conversation');
    if (!conversationEl) return;

    let isNearBottom = false;
    if (isAutoRefresh) {
        isNearBottom = conversationEl.scrollHeight - conversationEl.scrollTop - conversationEl.clientHeight < 100;
    }

    if (!isAutoRefresh) {
        conversationEl.innerHTML = '<div class="loading">Loading conversation...</div>';
    }

    try {
        const resp = await fetch(`/api/session/details?agentId=${agentId}&sessionId=${sessionId}`);
        if (!resp.ok) throw new Error("Failed to load session details");
        const data = await resp.json();
        
        renderConversation(data.turns);

        if (!isAutoRefresh) {
            // Initial load: always scroll to bottom
            setTimeout(() => {
                conversationEl.scrollTop = conversationEl.scrollHeight;
            }, 50);
        } else if (isNearBottom) {
            // Auto-refresh: only scroll if was already at bottom
            conversationEl.scrollTop = conversationEl.scrollHeight;
        }
    } catch (err) {
        console.error(err);
        if (!isAutoRefresh) {
            conversationEl.innerHTML = `<div class="error">Error: ${err.message}</div>`;
        }
    }
}

function renderConversation(turns) {
    const conversationEl = document.getElementById('modal-conversation');
    
    if (!turns || turns.length === 0) {
        conversationEl.innerHTML = '<div class="empty-state">No messages in this session.</div>';
        return;
    }

    conversationEl.innerHTML = turns.map((turn, idx) => `
        <div class="conversation-turn" id="turn-${idx}">
            <div class="detail-section">
                <label>User Message</label>
                <div class="message-box user">${turn.user_message || '(No message)'}</div>
            </div>
            ${turn.reasoning ? `
            <div class="detail-section">
                <label>LLM Reasoning</label>
                <div class="message-box reasoning" id="reasoning-${idx}">${turn.reasoning}</div>
            </div>
            ` : `<div id="reasoning-${idx}"></div>`}
            <div class="detail-section">
                <label>OpenClaw Response</label>
                <div class="message-box assistant">${turn.final_text || '...'}</div>
            </div>
            ${turn.input_tokens > 0 || turn.cost > 0 ? `
            <div class="turn-usage">
                <span>Tokens: <strong>${(turn.input_tokens + turn.output_tokens).toLocaleString()}</strong> (In: ${turn.input_tokens.toLocaleString()}, Out: ${turn.output_tokens.toLocaleString()})</span>
                <span>Cost: <strong>$${turn.cost.toFixed(4)}</strong></span>
            </div>
            ` : ''}
        </div>
    `).join('<hr class="turn-divider">');
}

function updateLiveReasoning(payload) {
    const { agent_id, delta } = payload;
    const modal = document.getElementById('detail-modal');
    if (modal && modal.style.display === 'flex' && modal.dataset.agentId === agent_id) {
        const conversationEl = document.getElementById('modal-conversation');
        if (!conversationEl) return;

        // Sticky scroll check
        const isNearBottom = conversationEl.scrollHeight - conversationEl.scrollTop - conversationEl.clientHeight < 50;

        // Find the LATEST reasoning box in the modal
        const reasoningBoxes = document.querySelectorAll('.message-box.reasoning');
        let latestReasoning = reasoningBoxes[reasoningBoxes.length - 1];
        
        if (!latestReasoning) {
            const turns = document.querySelectorAll('.conversation-turn');
            const lastTurn = turns[turns.length - 1];
            if (lastTurn) {
                const target = lastTurn.querySelector('[id^="reasoning-"]');
                if (target && !target.classList.contains('reasoning')) {
                    target.className = 'message-box reasoning';
                    latestReasoning = target;
                }
            }
        }

        if (latestReasoning) {
            latestReasoning.textContent += delta;
            if (isNearBottom) {
                conversationEl.scrollTop = conversationEl.scrollHeight;
            }
        }
    }
}

function closeDetailModal() {
    const modal = document.getElementById('detail-modal');
    if (modal) modal.style.display = 'none';
}
