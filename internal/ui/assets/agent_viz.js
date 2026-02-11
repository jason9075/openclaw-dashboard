
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

let currentPersonas = [];

function renderAgents(data) {
    const listContainer = document.getElementById('full-agent-list');
    const personas = data.personas || [];
    currentPersonas = personas;
    const runs = data.sub_agents || [];

    if (personas.length === 0 && runs.length === 0) {
        listContainer.innerHTML = '<div class="empty-state">No agents or activity found.</div>';
        return;
    }

    // Sort Personas: Default Agent (Fixed at 0), then Thinking, then by LastActive (descending)
    personas.sort((a, b) => {
        if (a.is_default && !b.is_default) return -1;
        if (!a.is_default && b.is_default) return 1;

        if (a.status === 'thinking' && b.status !== 'thinking') return -1;
        if (a.status !== 'thinking' && b.status === 'thinking') return 1;
        
        if (a.last_active !== b.last_active) return b.last_active - a.last_active;
        return a.id.localeCompare(b.id);
    });

    let html = '';

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
                        <h3>${p.name} ${p.is_default ? '<span class="default-label">(Default)</span>' : ''}</h3>
                        <div class="persona-sub">
                            <code>ID: ${p.id}</code>
                            <span class="model-badge">🤖 ${p.model || 'Default'}</span>
                        </div>
                    </div>
                    <div class="header-icons">
                        <div class="path-tooltip-wrapper">
                            <button class="btn-icon-top" onclick="copyPath(event, '${p.agent_dir}')" title="Click to copy path">📂
                                <div class="path-tooltip">
                                    <div class="path-item"><strong>Agent Root:</strong> <code>${p.agent_dir}</code></div>
                                    <div class="path-item copy-status" style="margin-top:5px; font-size:0.75rem; color:var(--nord8);">Click to copy path</div>
                                </div>
                            </button>
                        </div>
                        <button class="btn-icon-top" onclick="openSkillsModal('${p.id}')" title="Skills">🎯</button>
                        <button class="btn-icon-top" onclick="openPermissionsModal('${p.id}')" title="Permissions">🛡</button>
                        <button class="btn-icon-top" onclick="openCronModal('${p.id}')" title="Cron Jobs">⏰</button>
                        <span class="badge-mini ${isThinking ? 'thinking' : 'online'}"></span>
                    </div>
                </div>

                <div class="persona-content">
                    <p class="agent-snippet">${p.snippet || 'No AGENTS.md description found.'}</p>
                </div>

                <div class="persona-footer">
                    <button class="btn-detail-bottom" onclick="openDetailModal('${p.id}')">Detail</button>
                </div>
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
    const totalsEl = document.getElementById('modal-session-totals');
    if (!conversationEl) return;

    let isNearBottom = false;
    if (isAutoRefresh) {
        isNearBottom = conversationEl.scrollHeight - conversationEl.scrollTop - conversationEl.clientHeight < 100;
    }

    if (!isAutoRefresh) {
        conversationEl.innerHTML = '<div class="loading">Loading conversation...</div>';
        if (totalsEl) totalsEl.innerHTML = '';
    }

    try {
        const resp = await fetch(`/api/session/details?agentId=${agentId}&sessionId=${sessionId}`);
        if (!resp.ok) throw new Error("Failed to load session details");
        const data = await resp.json();
        
        renderConversation(data.turns);

        if (totalsEl) {
            totalsEl.innerHTML = `
                <span>Total Tokens: <strong>${(data.total_tokens || 0).toLocaleString()}</strong></span>
                ${data.total_cache_read > 0 ? `<span>Cached (Docs): <strong>${data.total_cache_read.toLocaleString()}</strong></span>` : ''}
                <span>Total Cost: <strong>$${(data.total_cost || 0).toFixed(4)}</strong></span>
            `;
        }

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

    conversationEl.innerHTML = turns.map((turn, idx) => {
        const timeStr = turn.timestamp ? new Date(turn.timestamp).toLocaleString() : '';
        const calledAgents = turn.agent_calls || [];
        const contextFiles = turn.context_files || [];
        const contextTokensEst = Math.round(turn.context_chars / 4);
        const isSystem = turn.user_source === 'system';

        return `
        <div class="conversation-turn ${isSystem ? 'is-system-turn' : ''}" id="turn-${idx}">
            <div class="detail-section">
                <label class="section-label-flex">
                    <span>${isSystem ? '🤖 OpenClaw System' : '👤 User Message'}</span>
                    <span class="turn-time">${timeStr}</span>
                </label>
                <div class="message-box user ${isSystem ? 'system' : ''}">${turn.user_message || '(No message)'}</div>
            </div>
            
            ${contextFiles.length > 0 ? `
            <div class="context-injected">
                <span class="label">📄 Injected Context (Docs):</span>
                <span class="docs-list">${contextFiles.join(', ')}</span>
                <span class="ctx-tag">+~${contextTokensEst.toLocaleString()} tokens</span>
            </div>
            ` : ''}

            ${calledAgents.length > 0 ? `
            <div class="agent-calls-container">
                <span class="label">📡 Called Agents:</span>
                <div class="call-tags">
                    ${calledAgents.map(a => `<button class="call-tag" onclick="jumpToAgent('${a}')">${a}</button>`).join('')}
                </div>
            </div>
            ` : ''}

            ${turn.reasoning ? `
            <div class="detail-section">
                <label>LLM Reasoning</label>
                <div class="message-box reasoning" id="reasoning-${idx}">${turn.reasoning}</div>
            </div>
            ` : `<div id="reasoning-${idx}"></div>`}
            <div class="detail-section">
                <label class="section-label-flex">
                    <span>OpenClaw Response</span>
                    <span class="turn-time">${timeStr}</span>
                </label>
                <div class="message-box assistant">${turn.final_text || '...'}</div>
            </div>
            ${turn.input_tokens > 0 || turn.cost > 0 ? `
            <div class="turn-usage">
                <span>Tokens: <strong>${(turn.input_tokens + turn.output_tokens).toLocaleString()}</strong> (In: ${turn.input_tokens.toLocaleString()}, Out: ${turn.output_tokens.toLocaleString()})</span>
                ${turn.cache_read_tokens > 0 ? `<span>Cached: <strong>${turn.cache_read_tokens.toLocaleString()}</strong></span>` : ''}
                <span>Cost: <strong>$${turn.cost.toFixed(4)}</strong></span>
            </div>
            ` : ''}
        </div>
        `;
    }).join('<hr class="turn-divider">');
}

function jumpToAgent(agentId) {
    closeDetailModal();
    const card = document.querySelector(`.agent-persona-card[data-id="${agentId}"]`);
    if (card) {
        card.scrollIntoView({ behavior: 'smooth', block: 'center' });
        card.classList.add('highlight-flash');
        setTimeout(() => card.classList.remove('highlight-flash'), 2000);
    } else {
        alert(`Agent "${agentId}" not found in current list.`);
    }
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

function copyPath(event, path) {
    event.stopPropagation();
    const btn = event.currentTarget;
    
    const showFeedback = (success) => {
        const originalText = btn.childNodes[0].nodeValue;
        btn.childNodes[0].nodeValue = success ? '✅' : '❌';
        const tooltip = btn.querySelector('.path-tooltip');
        if (tooltip) {
            const statusMsg = tooltip.querySelector('.copy-status');
            if (statusMsg) statusMsg.textContent = success ? "Copied!" : "Error";
        }
        setTimeout(() => {
            btn.childNodes[0].nodeValue = originalText;
            if (tooltip) {
                const statusMsg = tooltip.querySelector('.copy-status');
                if (statusMsg) statusMsg.textContent = "Click to copy path";
            }
        }, 1500);
    };

    // Use a more reliable fallback for copying
    const copyToClipboard = (text) => {
        if (navigator.clipboard && window.isSecureContext) {
            return navigator.clipboard.writeText(text);
        } else {
            const textArea = document.createElement("textarea");
            textArea.value = text;
            textArea.style.position = "fixed";
            textArea.style.left = "-9999px";
            textArea.style.top = "0";
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();
            return new Promise((res, rej) => {
                document.execCommand('copy') ? res() : rej();
                document.body.removeChild(textArea);
            });
        }
    };

    copyToClipboard(path)
        .then(() => showFeedback(true))
        .catch(() => showFeedback(false));
}

function closeModal(id) {
    const modal = document.getElementById(id);
    if (modal) modal.style.display = 'none';
}

function openSkillsModal(agentId) {
    const persona = currentPersonas.find(p => p.id === agentId);
    if (!persona) return;

    document.getElementById('modal-skills-agent').textContent = persona.name;
    const listEl = document.getElementById('modal-skills-list');
    
    if (!persona.skills || persona.skills.length === 0) {
        listEl.innerHTML = '<div class="empty-state">No specific skills enabled for this agent.</div>';
    } else {
        listEl.innerHTML = persona.skills.map(s => `<span class="skill-tag-lg">${s}</span>`).join('');
    }
    
    document.getElementById('skills-modal').style.display = 'flex';
}

function openPermissionsModal(agentId) {
    const persona = currentPersonas.find(p => p.id === agentId);
    if (!persona) return;

    document.getElementById('modal-perm-agent').textContent = persona.name;
    document.getElementById('modal-perm-profile').textContent = persona.tool_profile || 'Default';
    
    let desc = "This profile defines which tools and system capabilities the agent can access.";
    if (persona.tool_profile === 'full') {
        desc = "Full Access Profile: Authorized to Read, Create, and Edit files. Can also use the web browser and execute system commands.";
    } else if (persona.tool_profile === 'coding') {
        desc = "Coding Profile: Optimized for development. Authorized to Read, Create, and Edit project files. Restricted system-level access.";
    } else if (persona.tool_profile === 'minimal') {
        desc = "Minimal Profile: Restricted access. Limited to basic assistance and Read-only operations in specific contexts.";
    }
    
    document.getElementById('modal-perm-desc').textContent = desc;
    document.getElementById('permissions-modal').style.display = 'flex';
}

function openCronModal(agentId) {
    const persona = currentPersonas.find(p => p.id === agentId);
    if (!persona) return;

    document.getElementById('modal-cron-agent').textContent = persona.name;
    const listEl = document.getElementById('modal-cron-list');
    
    if (!persona.cron_jobs || persona.cron_jobs.length === 0) {
        listEl.innerHTML = '<div class="empty-state">No scheduled jobs for this agent.</div>';
    } else {
        listEl.innerHTML = persona.cron_jobs.map(j => `
            <div class="cron-detailed-card ${j.enabled ? 'active' : 'disabled'}">
                <div class="cron-header">
                    <strong>${j.name}</strong>
                    <span class="status-dot"></span>
                </div>
                <p class="cron-desc">${j.description || 'No description provided.'}</p>
                <div class="cron-meta">
                    <div class="meta-item"><span>Schedule:</span> <code>${j.schedule}</code></div>
                    <div class="meta-item"><span>Next Run:</span> <code>${j.next_run || 'Not scheduled'}</code></div>
                </div>
            </div>
        `).join('');
    }
    
    document.getElementById('cron-modal').style.display = 'flex';
}
