package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type TodoItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

type Session struct {
	ID        string `json:"id"`
	User      string `json:"user"`
	RemoteIP  string `json:"remote_ip"`
	StartedAt string `json:"started_at"`
}

type Alert struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Time    string `json:"time"`
}

type GitCommit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
}

type CronJob struct {
	ID          string `json:"id"`
	AgentID     string `json:"agent_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Schedule    string `json:"schedule"`
	NextRun     string `json:"next_run"`
}

type CostCard struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type TokenStats struct {
	Model        string  `json:"model"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
}

type SubAgentRun struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	Duration string  `json:"duration"`
	Cost     float64 `json:"cost"`
	Tokens   int     `json:"tokens"`
	ID       string  `json:"id,omitempty"`
}

type PersistedSubagentRegistry struct {
	Version int                          `json:"version"`
	Runs    map[string]SubagentRunRecord `json:"runs"`
}

type SubagentRunRecord struct {
	RunID     string              `json:"runId"`
	Task      string              `json:"task"`
	Label     string              `json:"label"`
	CreatedAt int64               `json:"createdAt"`
	StartedAt int64               `json:"startedAt"`
	EndedAt   int64               `json:"endedAt"`
	Outcome   *SubagentRunOutcome `json:"outcome"`
}

type SubagentRunOutcome struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}

type Model struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type AgentPersona struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Emoji       string    `json:"emoji"`
	Workspace   string    `json:"workspace"`
	AgentDir    string    `json:"agent_dir"`
	Sessions    string    `json:"sessions_dir"`
	IsDefault   bool      `json:"is_default"`
	Skills      []string  `json:"skills"`
	Status      string    `json:"status"`
	LastActive  int64     `json:"last_active"`
	Model       string    `json:"model"`
	ToolProfile string    `json:"tool_profile"`
	CronJobs    []CronJob `json:"cron_jobs"`
	Snippet     string    `json:"snippet"`
}

type AgentCall struct {
	AgentID   string `json:"agent_id"`
	SessionID string `json:"session_id"`
}

type ConversationTurn struct {
	UserMessage     string      `json:"user_message"`
	UserSource      string      `json:"user_source"` // "user", "system"
	Reasoning       string      `json:"reasoning"`
	FinalText       string      `json:"final_text"`
	InputTokens     int64       `json:"input_tokens"`
	OutputTokens    int64       `json:"output_tokens"`
	CacheReadTokens int64       `json:"cache_read_tokens"`
	Cost            float64     `json:"cost"`
	Timestamp       string      `json:"timestamp"`
	AgentCalls      []AgentCall `json:"agent_calls"`
	ContextFiles    []string    `json:"context_files"`
	ContextChars    int64       `json:"context_chars"`
}

type SessionDetails struct {
	AgentID        string             `json:"agent_id"`
	SessionID      string             `json:"session_id"`
	Turns          []ConversationTurn `json:"turns"`
	TotalTokens    int64              `json:"total_tokens"`
	TotalCacheRead int64              `json:"total_cache_read"`
	TotalCost      float64            `json:"total_cost"`
}

type SessionInfo struct {
	ID        string    `json:"id"`
	UpdatedAt time.Time `json:"updated_at"`
}

type DataProvider struct {
	BasePath       string
	sessionToAgent map[string]string
	agentStatus    map[string]string
	agentReasoning map[string]string
	lastUpdate     map[string]time.Time
	mu             sync.RWMutex
}

func NewDataProvider() (*DataProvider, error) {
	basePath := os.Getenv("OPENCLAW_STATE_DIR")
	if basePath == "" { basePath = os.Getenv("CLAWDBOT_STATE_DIR") }
	if basePath == "" {
		candidates := []string{".openclaw", "/home/jason9075/.openclaw", "/home/clawbot/.openclaw"}
		entries, _ := os.ReadDir("/home")
		for _, e := range entries { if e.IsDir() { candidates = append(candidates, filepath.Join("/home", e.Name(), ".openclaw")) } }
		for _, c := range candidates { if _, err := os.Stat(filepath.Join(c, "openclaw.json")); err == nil { basePath = c; break } }
		if basePath == "" { if home, err := os.UserHomeDir(); err == nil { basePath = filepath.Join(home, ".openclaw") } }
	}
	if _, err := os.Stat(basePath); os.IsNotExist(err) { _ = os.MkdirAll(basePath, 0755) }
	dp := &DataProvider{ BasePath: basePath, sessionToAgent: make(map[string]string), agentStatus: make(map[string]string), agentReasoning: make(map[string]string), lastUpdate: make(map[string]time.Time) }
	dp.RefreshSessionMap()
	return dp, nil
}

func (dp *DataProvider) expandPath(p string) string {
	if strings.HasPrefix(p, "~") { home, err := os.UserHomeDir(); if err == nil { return filepath.Join(home, p[1:]) } }
	return p
}

func (dp *DataProvider) normalizeOpenClawPath(p string) string {
	p = dp.expandPath(p)
	if strings.Contains(p, ".openclaw") {
		parts := strings.SplitN(p, ".openclaw", 2); if len(parts) == 2 { newPath := filepath.Join(dp.BasePath, parts[1]); if _, err := os.Stat(newPath); err == nil { return newPath } }
	}
	return p
}

func (dp *DataProvider) MarkAgentActive(agentID string, status string) {
	dp.mu.Lock(); defer dp.mu.Unlock(); dp.agentStatus[agentID] = status; dp.lastUpdate[agentID] = time.Now()
}

func (dp *DataProvider) RefreshSessionMap() {
	dp.mu.Lock(); defer dp.mu.Unlock(); dp.sessionToAgent = make(map[string]string)
	agentsDir := filepath.Join(dp.BasePath, "agents"); entries, _ := os.ReadDir(agentsDir)
	for _, agentEntry := range entries {
		if !agentEntry.IsDir() { continue }
		agentID := agentEntry.Name(); sessionsDir := filepath.Join(agentsDir, agentID, "sessions"); sessions, _ := os.ReadDir(sessionsDir)
		for _, s := range sessions { if !s.IsDir() && filepath.Ext(s.Name()) == ".jsonl" { dp.sessionToAgent[strings.TrimSuffix(s.Name(), ".jsonl")] = agentID } }
	}
}

func (dp *DataProvider) WatchRawStream(broadcaster *Broadcaster) {
	logPath := filepath.Join(dp.BasePath, "logs", "raw-stream.jsonl"); _ = os.MkdirAll(filepath.Dir(logPath), 0755)
	watcher, err := fsnotify.NewWatcher(); if err != nil { return }; _ = watcher.Add(filepath.Dir(logPath))
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			dp.mu.Lock(); now := time.Now()
			for agentID, last := range dp.lastUpdate {
				if dp.agentStatus[agentID] == "thinking" && now.Sub(last) > 3*time.Second {
					dp.agentStatus[agentID] = "idle"; dp.agentReasoning[agentID] = ""
					broadcaster.Broadcast(Event{Type: "agent_status", Payload: map[string]string{"agent_id": agentID, "status": "idle"}})
				}
			}
			dp.mu.Unlock()
		}
	}()
	go func() {
		defer watcher.Close(); var lastSize int64; if info, err := os.Stat(logPath); err == nil { lastSize = info.Size() }
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok { return }
				if event.Name == logPath && (event.Op&fsnotify.Write == fsnotify.Write) {
					info, err := os.Stat(logPath); if err == nil && info.Size() > lastSize { dp.processRawStreamNewData(logPath, lastSize, broadcaster); lastSize = info.Size() }
				}
			}
		}
	}()
}

func (dp *DataProvider) processRawStreamNewData(path string, offset int64, broadcaster *Broadcaster) {
	file, err := os.Open(path); if err != nil { return }; defer file.Close(); _, _ = file.Seek(offset, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry struct { Event string `json:"event"`; SessionID string `json:"sessionId"`; Delta string `json:"delta"` }
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil { continue }; if entry.SessionID == "" { continue }
		dp.mu.RLock(); agentID, ok := dp.sessionToAgent[entry.SessionID]; dp.mu.RUnlock()
		if !ok { dp.RefreshSessionMap(); dp.mu.RLock(); agentID = dp.sessionToAgent[entry.SessionID]; dp.mu.RUnlock() }
		if agentID != "" {
			dp.mu.Lock(); dp.agentStatus[agentID] = "thinking"; dp.lastUpdate[agentID] = time.Now()
			if entry.Delta != "" { dp.agentReasoning[agentID] += entry.Delta; broadcaster.Broadcast(Event{Type: "reasoning_delta", Payload: map[string]string{"agent_id": agentID, "delta": entry.Delta}}) }
			dp.mu.Unlock()
			broadcaster.Broadcast(Event{Type: "agent_status", Payload: map[string]string{"agent_id": agentID, "status": "thinking"}})
		}
	}
}

func (dp *DataProvider) GetSessionsForAgent(agentID string) ([]SessionInfo, error) {
	personas, _ := dp.GetAgentPersonas()
	var sessionsDir string
	for _, p := range personas { if p.ID == agentID { sessionsDir = p.Sessions; break } }
	if sessionsDir == "" { sessionsDir = dp.normalizeOpenClawPath(filepath.Join(dp.BasePath, "agents", agentID, "sessions")) }
	entries, err := os.ReadDir(sessionsDir); if err != nil { if os.IsNotExist(err) { return []SessionInfo{}, nil }; return nil, err }
	var sessions []SessionInfo
	for _, e := range entries { if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" { info, _ := e.Info(); sessions = append(sessions, SessionInfo{ID: strings.TrimSuffix(e.Name(), ".jsonl"), UpdatedAt: info.ModTime()}) } }
	return sessions, nil
}

func (dp *DataProvider) GetSessionDetails(agentID string, sessionID string) (*SessionDetails, error) {
	personas, _ := dp.GetAgentPersonas()
	var sessionsDir string
	for _, p := range personas { if p.ID == agentID { sessionsDir = p.Sessions; break } }
	if sessionsDir == "" { sessionsDir = dp.normalizeOpenClawPath(filepath.Join(dp.BasePath, "agents", agentID, "sessions")) }
	sessionFile := filepath.Join(sessionsDir, sessionID+".jsonl")
	file, err := os.Open(sessionFile); if err != nil { return nil, err }; defer file.Close()
	details := &SessionDetails{AgentID: agentID, SessionID: sessionID, Turns: []ConversationTurn{}}
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 64*1024); scanner.Buffer(buf, 1024*1024)
	var currentTurn *ConversationTurn; var reasoningBuilder strings.Builder
	for scanner.Scan() {
		rawLine := scanner.Bytes()
		var meta struct { Timestamp string `json:"timestamp"`; Message struct { Role string `json:"role"` } `json:"message"` }
		if err := json.Unmarshal(rawLine, &meta); err != nil { continue }
		if meta.Message.Role == "user" {
			if currentTurn != nil { currentTurn.Reasoning = reasoningBuilder.String(); details.Turns = append(details.Turns, *currentTurn); reasoningBuilder.Reset() }
			ctxFiles, ctxChars := dp.estimateContextStats(agentID)
			currentTurn = &ConversationTurn{Timestamp: meta.Timestamp, ContextFiles: ctxFiles, ContextChars: ctxChars, UserSource: "user"}
			var contentLine struct { Message struct { Content interface{} `json:"content"` } `json:"message"` }
			if err := json.Unmarshal(rawLine, &contentLine); err == nil {
				if content, ok := contentLine.Message.Content.(string); ok { currentTurn.UserMessage = content
				} else if contentArr, ok := contentLine.Message.Content.([]interface{}); ok {
					for _, part := range contentArr { if p, ok := part.(map[string]interface{}); ok && p["type"] == "text" { if val, ok := p["text"].(string); ok { currentTurn.UserMessage = val } } }
				}
			}
			msg := strings.TrimSpace(currentTurn.UserMessage)
			if strings.HasPrefix(msg, "A new session was started") || strings.HasPrefix(msg, "HEARTBEAT") || strings.HasPrefix(msg, "System: [") || strings.Contains(msg, "configured persona") || strings.Contains(msg, "An async command you ran earlier has completed") { currentTurn.UserSource = "system" }
		} else if meta.Message.Role == "assistant" && currentTurn != nil {
			var assistLine struct { Message struct { Content interface{} `json:"content"`; Usage struct { Input int64 `json:"input"`; Output int64 `json:"output"`; CacheRead int64 `json:"cacheRead"`; Cost struct { Total float64 `json:"total"` } `json:"cost"` } `json:"usage"` } `json:"message"` }
			if err := json.Unmarshal(rawLine, &assistLine); err == nil {
				u := assistLine.Message.Usage; currentTurn.InputTokens += u.Input; currentTurn.OutputTokens += u.Output; currentTurn.CacheReadTokens += u.CacheRead; currentTurn.Cost += u.Cost.Total
				details.TotalTokens += (u.Input + u.Output); details.TotalCacheRead += u.CacheRead; details.TotalCost += u.Cost.Total
				currentTurn.Timestamp = meta.Timestamp
				if content, ok := assistLine.Message.Content.(string); ok { currentTurn.FinalText = content
					think := extractThinkingFromTaggedText(content); if think != "" { reasoningBuilder.WriteString(think); currentTurn.FinalText = stripThinkingTagsFromText(content) }
				} else if contentArr, ok := assistLine.Message.Content.([]interface{}); ok {
					for _, part := range contentArr { if p, ok := part.(map[string]interface{}); ok { if p["type"] == "thinking" { if val, ok := p["thinking"].(string); ok { reasoningBuilder.WriteString(val) } } else if p["type"] == "text" { if val, ok := p["text"].(string); ok { currentTurn.FinalText += val } } } }
				}
			}
		} else if (meta.Message.Role == "toolResult" || meta.Message.Role == "tool_result") && currentTurn != nil {
			var result struct { Message struct { Details struct { ChildSessionKey string `json:"childSessionKey"`; AgentID string `json:"agentId"` } `json:"details"` } `json:"message"` }
			if err := json.Unmarshal(rawLine, &result); err == nil && result.Message.Details.ChildSessionKey != "" {
				parts := strings.Split(result.Message.Details.ChildSessionKey, ":"); childSessID := parts[len(parts)-1]; childAgentID := result.Message.Details.AgentID
				if childAgentID == "" && len(parts) >= 2 { childAgentID = parts[1] }
				currentTurn.AgentCalls = append(currentTurn.AgentCalls, AgentCall{AgentID: childAgentID, SessionID: childSessID})
			}
		}
	}
	if currentTurn != nil {
		currentTurn.Reasoning = reasoningBuilder.String(); dp.mu.RLock(); if dp.agentStatus[agentID] == "thinking" && dp.agentReasoning[agentID] != "" { currentTurn.Reasoning = dp.agentReasoning[agentID] }; dp.mu.RUnlock(); details.Turns = append(details.Turns, *currentTurn)
	}
	return details, nil
}

func (dp *DataProvider) estimateContextStats(agentID string) ([]string, int64) {
	stdFiles := []string{"SOUL.md", "IDENTITY.md", "AGENTS.md", "USER.md", "TOOLS.md", "HEARTBEAT.md"}
	var foundFiles []string; var totalChars int64
	workspace := filepath.Join(dp.BasePath, "agents", agentID, "workspace")
	if _, err := os.Stat(workspace); err != nil && agentID == "main" { workspace = filepath.Join(dp.BasePath, "workspace") }
	for _, name := range stdFiles { p := filepath.Join(workspace, name); if info, err := os.Stat(p); err == nil { foundFiles = append(foundFiles, name); totalChars += info.Size() } }
	return foundFiles, totalChars
}

func stripThinkingTagsFromText(text string) string {
	re := []string{"<think>", "</think>", "<thinking>", "</thinking>", "<thought>", "</thought>"}
	for _, r := range re { text = strings.ReplaceAll(text, r, "") }
	return strings.TrimSpace(text)
}

func extractThinkingFromTaggedText(text string) string {
	tags := [][]string{{"<think>", "</think>"}, {"<thinking>", "</thinking>"}, {"<thought>", "</thought>"}}
	for _, t := range tags { start := strings.Index(text, t[0]); end := strings.Index(text, t[1]); if start != -1 && end != -1 && end > start { return text[start+len(t[0]) : end] } }
	return ""
}

func (dp *DataProvider) GetSubagentRetention() int {
	configPath := filepath.Join(dp.BasePath, "openclaw.json"); data, err := os.ReadFile(configPath); if err != nil { return 60 }
	var cfg struct { Agents struct { Defaults struct { Subagents struct { ArchiveAfterMinutes int `json:"archiveAfterMinutes"` } `json:"subagents"` } `json:"defaults"` } `json:"agents"` }
	if err := json.Unmarshal(data, &cfg); err != nil { return 60 }; if cfg.Agents.Defaults.Subagents.ArchiveAfterMinutes == 0 { return 60 }
	return cfg.Agents.Defaults.Subagents.ArchiveAfterMinutes
}

func (dp *DataProvider) GetAgentPersonas() ([]AgentPersona, error) {
	externalSkills := make(map[string]bool); skills, _ := dp.GetSkills(); for _, s := range skills { externalSkills[s.Name] = true }
	allCronJobs, _ := dp.GetCronJobs(); configPath := filepath.Join(dp.BasePath, "openclaw.json"); data, err := os.ReadFile(configPath); if err != nil { return nil, err }
	var cfg struct { Agents struct { List []struct { ID string `json:"id"`; Name string `json:"name"`; Workspace string `json:"workspace"`; AgentDir string `json:"agentDir"`; Default bool `json:"default"`; Skills []string `json:"skills"`; Model struct { Primary string `json:"primary"` } `json:"model"`; Identity struct { Emoji string `json:"emoji"` } `json:"identity"`; Tools struct { Profile string `json:"profile"` } `json:"tools"` } `json:"list"` } `json:"agents"` }
	if err := json.Unmarshal(data, &cfg); err != nil { return nil, err }
	var personas []AgentPersona
	for idx, a := range cfg.Agents.List {
		id := a.ID; if id == "" { continue }
		workspace := dp.normalizeOpenClawPath(a.Workspace)
		if workspace == "" { if id == "main" { workspace = filepath.Join(dp.BasePath, "workspace") } else { workspace = filepath.Join(dp.BasePath, "workspace-"+id) } }
		sessionsDir := dp.normalizeOpenClawPath(filepath.Join(dp.BasePath, "agents", id, "sessions"))
		name := a.Name; if name == "" { name = strings.Title(id) }
		emoji := a.Identity.Emoji; if emoji == "" { emoji = dp.readEmojiFromIdentity(workspace) }
		if emoji == "" { lid := strings.ToLower(id); if strings.Contains(lid, "coder") { emoji = "💻" } else if strings.Contains(lid, "research") { emoji = "🔍" } else if strings.Contains(lid, "schedule") { emoji = "📅" } else { emoji = "🧠" } }
		var filteredSkills []string; if a.Skills != nil { for _, s := range a.Skills { if externalSkills[s] { filteredSkills = append(filteredSkills, s) } } } else { filteredSkills = dp.getSkillNames(externalSkills) }
		dp.mu.RLock(); status := dp.agentStatus[id]; if status == "" { status = "idle" }; lastActive := dp.lastUpdate[id].Unix(); dp.mu.RUnlock()
		var agentCrons []CronJob; for _, c := range allCronJobs { if c.AgentID == id { agentCrons = append(agentCrons, c) } }
		snippet := dp.readCapabilitiesSnippet(workspace); agentRoot := filepath.Dir(workspace); isDefault := a.Default; if idx == 0 { isDefault = true }
		personas = append(personas, AgentPersona{ ID: id, Name: name, Emoji: emoji, Workspace: workspace, AgentDir: agentRoot, Sessions: sessionsDir, IsDefault: isDefault, Skills: filteredSkills, Status: status, LastActive: lastActive, Model: a.Model.Primary, ToolProfile: a.Tools.Profile, CronJobs: agentCrons, Snippet: snippet })
	}
	return personas, nil
}

func (dp *DataProvider) GetAgentPersonasInternal() ([]AgentPersona, error) {
	configPath := filepath.Join(dp.BasePath, "openclaw.json"); data, err := os.ReadFile(configPath); if err != nil { return nil, nil }
	var cfg struct { Agents struct { List []struct { ID string `json:"id"`; Workspace string `json:"workspace"` } `json:"list"` } `json:"agents"` }
	if err := json.Unmarshal(data, &cfg); err != nil { return nil, err }
	var personas []AgentPersona
	for _, a := range cfg.Agents.List { workspace := dp.normalizeOpenClawPath(a.Workspace); if workspace == "" { if a.ID == "main" { workspace = filepath.Join(dp.BasePath, "workspace") } else { workspace = filepath.Join(dp.BasePath, "workspace-"+a.ID) } }; personas = append(personas, AgentPersona{Workspace: workspace}) }
	return personas, nil
}

func (dp *DataProvider) getSkillNames(m map[string]bool) []string { var names []string; for k := range m { names = append(names, k) }; return names }

func (dp *DataProvider) readEmojiFromIdentity(workspace string) string {
	path := filepath.Join(workspace, "IDENTITY.md"); data, err := os.ReadFile(path); if err != nil { return "" }
	lines := strings.Split(string(data), "\n"); for _, line := range lines { trimmed := strings.TrimSpace(line); if strings.Contains(trimmed, "Emoji:") { parts := strings.SplitN(trimmed, ":", 2); if len(parts) == 2 { emoji := strings.TrimSpace(parts[1]); emoji = strings.Trim(emoji, "*"); return strings.TrimSpace(emoji) } } }
	return ""
}

func (dp *DataProvider) readCapabilitiesSnippet(workspace string) string {
	path := filepath.Join(workspace, "IDENTITY.md"); data, err := os.ReadFile(path); if err != nil { return "" }
	lines := strings.Split(string(data), "\n"); found := false; var snippetBuilder strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line); if strings.Contains(trimmed, "## Capabilities") { found = true; continue }
		if found {
			if strings.HasPrefix(trimmed, "##") { break }
			if trimmed != "" {
				text := trimmed; for { changed := false; if strings.HasPrefix(text, "-") { text = text[1:]; changed = true }; if strings.HasPrefix(text, "*") { text = text[1:]; changed = true }; if strings.HasPrefix(text, " ") { text = text[1:]; changed = true }; if !changed { break } }
				if text != "" { text = strings.ReplaceAll(text, "**", ""); if snippetBuilder.Len() > 0 { snippetBuilder.WriteString(" ") }; snippetBuilder.WriteString(text); if snippetBuilder.Len() > 150 { break } }
			}
		}
	}
	runes := []rune(snippetBuilder.String()); if len(runes) > 50 { return string(runes[:50]) + "..." }; return string(runes)
}

func (dp *DataProvider) GetSubAgentActivity() ([]SubAgentRun, error) {
	var allRuns []SubAgentRun; legacyPath := filepath.Join(dp.BasePath, "subagents.json")
	if data, err := os.ReadFile(legacyPath); err == nil { var runs []SubAgentRun; if err := json.Unmarshal(data, &runs); err == nil { allRuns = append(allRuns, runs...) } }
	path := filepath.Join(dp.BasePath, "subagents", "runs.json")
	if data, err := os.ReadFile(path); err == nil {
		var registry PersistedSubagentRegistry; if err := json.Unmarshal(data, &registry); err != nil {
			for _, record := range registry.Runs {
				name := record.Label; if name == "" { name = record.Task }; status := "running"
				if record.Outcome != nil { if record.Outcome.Status == "ok" { status = "completed" } else if record.Outcome.Status == "error" { status = "failed" } }
				duration := "--"; if record.StartedAt > 0 { var d time.Duration; if record.EndedAt > 0 { d = time.Duration(record.EndedAt-record.StartedAt) * time.Millisecond } else { d = time.Since(time.Unix(0, record.StartedAt*int64(time.Millisecond))) }; duration = d.Round(time.Second).String() }
				tokens, cost := dp.findUsageForRun(record.RunID)
				allRuns = append(allRuns, SubAgentRun{ID: record.RunID, Name: name, Status: status, Duration: duration, Tokens: int(tokens), Cost: cost})
			}
		}
	}
	return allRuns, nil
}

func (dp *DataProvider) findUsageForRun(runID string) (int64, float64) {
	pattern := filepath.Join(dp.BasePath, "agents", "*", "sessions", "*.jsonl"); files, _ := filepath.Glob(pattern)
	var totalTokens int64; var totalCost float64
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), runID) {
			file, err := os.Open(f); if err == nil {
				scanner := bufio.NewScanner(file); for scanner.Scan() {
					var line struct { Message struct { Role  string `json:"role"`; Usage struct { Input  int64 `json:"input"`; Output int64 `json:"output"`; Cost   struct { Total float64 `json:"total"` } `json:"cost"` } `json:"usage"` } `json:"message"` }
					if err := json.Unmarshal(scanner.Bytes(), &line); err == nil && line.Message.Role == "assistant" { totalTokens += (line.Message.Usage.Input + line.Message.Usage.Output); totalCost += line.Message.Usage.Cost.Total }
				}
				file.Close()
			}
		}
	}
	return totalTokens, totalCost
}

func (dp *DataProvider) WatchTranscripts(broadcaster *Broadcaster) error {
	watcher, err := fsnotify.NewWatcher(); if err != nil { return err }
	personas, _ := dp.GetAgentPersonas()
	for _, p := range personas { if _, err := os.Stat(p.Sessions); err == nil { _ = watcher.Add(p.Sessions) } }
	go func() {
		defer watcher.Close()
		for { select { case event, ok := <-watcher.Events: if !ok { return }; if (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) && filepath.Ext(event.Name) == ".jsonl" { dp.handleTranscriptChange(event.Name, broadcaster) } } }
	}()
	return nil
}

func (dp *DataProvider) handleTranscriptChange(path string, broadcaster *Broadcaster) {
	file, err := os.Open(path); if err != nil { return }; defer file.Close(); stat, _ := file.Stat()
	if stat.Size() < 2000 { _, _ = file.Seek(0, 0) } else { _, _ = file.Seek(-2000, 2) }
	scanner := bufio.NewScanner(file); var lastLine string; for scanner.Scan() { lastLine = scanner.Text() }; if lastLine == "" { return }
	var entry struct { Message struct { Role    string `json:"role"`; Content []struct { Type string `json:"type"`; Name string `json:"name"` } `json:"content"`; ToolCalls []struct { Function struct { Name string `json:"name"` } `json:"function"` } `json:"tool_calls"` } `json:"message"` }
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil { return }
	agentID := filepath.Base(filepath.Dir(filepath.Dir(path))); dp.MarkAgentActive(agentID, "thinking")
	broadcaster.Broadcast(Event{Type: "agent_status", Payload: map[string]string{"agent_id": agentID, "status": "thinking"}})
	for _, c := range entry.Message.Content { if c.Type == "tool_use" || c.Type == "tool_call" { broadcaster.Broadcast(Event{Type: "skill_use", Payload: map[string]string{"agent_id": agentID, "skill": c.Name, "status": "executing"}}); return } }
	for _, tc := range entry.Message.ToolCalls { if tc.Function.Name != "" { broadcaster.Broadcast(Event{Type: "skill_use", Payload: map[string]string{"agent_id": agentID, "skill": tc.Function.Name, "status": "executing"}}); return } }
	broadcaster.Broadcast(Event{Type: "refresh", Payload: map[string]string{"reason": "transcript_update", "agent_id": agentID}})
}

func (dp *DataProvider) ListFiles() ([]string, error) {
	var files []string; _ = filepath.Walk(dp.BasePath, func(path string, info os.FileInfo, err error) error { if err == nil && !info.IsDir() { rel, _ := filepath.Rel(dp.BasePath, path); files = append(files, rel) }; return nil }); return files, nil
}

func (dp *DataProvider) GetTodos() ([]TodoItem, error) {
	path := filepath.Join(dp.BasePath, "todo.json"); if data, err := os.ReadFile(path); err == nil { var todos []TodoItem; _ = json.Unmarshal(data, &todos); return todos, nil }; return []TodoItem{}, nil
}

func (dp *DataProvider) GetActiveSessions() (map[string]Session, error) {
	path := filepath.Join(dp.BasePath, "sessions", "active.json"); if data, err := os.ReadFile(path); err == nil { var sessions map[string]Session; _ = json.Unmarshal(data, &sessions); return sessions, nil }; return map[string]Session{}, nil
}

func (dp *DataProvider) GetAlerts() ([]Alert, error) {
	path := filepath.Join(dp.BasePath, "logs", "error.log"); file, err := os.Open(path); if err != nil { return []Alert{}, nil }; defer file.Close(); var alerts []Alert; scanner := bufio.NewScanner(file); var lines []string; for scanner.Scan() { lines = append(lines, scanner.Text()) }; start := 0; if len(lines) > 5 { start = len(lines) - 5 }; for _, line := range lines[start:] { parts := strings.SplitN(line, " - ", 2); if len(parts) == 2 { meta := strings.Fields(parts[0]); if len(meta) >= 2 { alerts = append(alerts, Alert{Level: strings.Trim(meta[0], "[]"), Time: strings.Join(meta[1:], " "), Message: parts[1]}) } } }; return alerts, nil
}

func (dp *DataProvider) GetGitLog() ([]GitCommit, error) {
	cmd := exec.Command("git", "log", "-n", "5", "--oneline"); output, err := cmd.Output(); if err != nil { return []GitCommit{}, nil }; var commits []GitCommit; for _, line := range strings.Split(string(output), "\n") { if parts := strings.SplitN(line, " ", 2); len(parts) == 2 { commits = append(commits, GitCommit{Hash: parts[0], Message: parts[1]}) } }; return commits, nil
}

func (dp *DataProvider) GetCronJobs() ([]CronJob, error) {
	path := filepath.Join(dp.BasePath, "cron", "jobs.json"); data, err := os.ReadFile(path); if err != nil { return []CronJob{}, nil }
	var raw struct { Jobs []struct { ID string `json:"id"`; AgentID string `json:"agentId"`; Name string `json:"name"`; Desc string `json:"description"`; Enabled bool `json:"enabled"`; Schedule struct { Kind string `json:"kind"`; Expr string `json:"expr"` } `json:"schedule"`; State struct { NextRunAtMs int64 `json:"nextRunAtMs"` } `json:"state"` } `json:"jobs"` }
	if err := json.Unmarshal(data, &raw); err != nil { return []CronJob{}, nil }; var jobs []CronJob; for _, j := range raw.Jobs { nextRun := ""; if j.State.NextRunAtMs > 0 { nextRun = time.Unix(j.State.NextRunAtMs/1000, 0).Format("2006-01-02 15:04") }; jobs = append(jobs, CronJob{ID: j.ID, AgentID: j.AgentID, Name: j.Name, Description: j.Desc, Enabled: j.Enabled, Schedule: j.Schedule.Kind + " " + j.Schedule.Expr, NextRun: nextRun}) }
	return jobs, nil
}

func (dp *DataProvider) GetCosts() ([]CostCard, error) {
	path := filepath.Join(dp.BasePath, "costs.json"); if data, err := os.ReadFile(path); err == nil { var costs []CostCard; _ = json.Unmarshal(data, &costs); return costs, nil }; return []CostCard{}, nil
}

func (dp *DataProvider) GetTokenUsage() ([]TokenStats, error) {
	path := filepath.Join(dp.BasePath, "tokens.json"); if data, err := os.ReadFile(path); err == nil { var stats []TokenStats; _ = json.Unmarshal(data, &stats); return stats, nil }; return []TokenStats{}, nil
}

func (dp *DataProvider) GetModels() ([]Model, error) {
	path := filepath.Join(dp.BasePath, "models.json"); if data, err := os.ReadFile(path); err == nil { var models []Model; if err := json.Unmarshal(data, &models); err != nil { return models, nil } }
	configPath := filepath.Join(dp.BasePath, "openclaw.json"); if data, err := os.ReadFile(configPath); err == nil { var cfg struct { Agents struct { Defaults struct { Models map[string]interface{} `json:"models"` } `json:"defaults"` } `json:"agents"` }; if err := json.Unmarshal(data, &cfg); err != nil { return nil, err }; var models []Model; for id := range cfg.Agents.Defaults.Models { models = append(models, Model{Name: id, Type: "chat"}) }; return models, nil }; return []Model{}, nil
}

func (dp *DataProvider) GetSkills() ([]Skill, error) {
	var allSkills []Skill; seen := make(map[string]bool); configPath := filepath.Join(dp.BasePath, "openclaw.json"); var extraDirs []string
	if data, err := os.ReadFile(configPath); err == nil { var cfg struct { Skills struct { Load struct { ExtraDirs []string `json:"extraDirs"` } `json:"load"` } `json:"skills"` }; if err := json.Unmarshal(data, &cfg); err != nil { extraDirs = cfg.Skills.Load.ExtraDirs } }
	managedPath := filepath.Join(dp.BasePath, "skills"); dp.scanSkillsDir(managedPath, &allSkills, seen)
	for _, dir := range extraDirs { dp.scanSkillsDir(dp.normalizeOpenClawPath(dir), &allSkills, seen) }
	personas, _ := dp.GetAgentPersonasInternal()
	for _, p := range personas { workspaceSkills := filepath.Join(p.Workspace, "skills"); dp.scanSkillsDir(workspaceSkills, &allSkills, seen) }
	return allSkills, nil
}

func (dp *DataProvider) scanSkillsDir(dir string, skills *[]Skill, seen map[string]bool) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() { continue }
		skillID := e.Name(); if seen[skillID] { continue }; skillPath := filepath.Join(dir, skillID, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			file, err := os.Open(skillPath); if err != nil { continue }; name := skillID; description := ""
			scanner := bufio.NewScanner(file); inFrontmatter := false
			for scanner.Scan() { line := scanner.Text(); if line == "---" { if !inFrontmatter { inFrontmatter = true; continue } else { break } }; if inFrontmatter { if strings.HasPrefix(line, "name:") { name = strings.TrimSpace(strings.TrimPrefix(line, "name:")) } else if strings.HasPrefix(line, "description:") { description = strings.TrimSpace(strings.TrimPrefix(line, "description:")); description = strings.Trim(description, "\"") } } }
			file.Close(); *skills = append(*skills, Skill{Name: name, Description: description}); seen[skillID] = true
		}
	}
}

type UsageBucket struct { Calls int; Input int64; Output int64; CacheRead int64; TotalTokens int64; Cost float64 }

func FriendlyModelName(model string) string {
	ml := strings.ToLower(model); if strings.Contains(ml, "opus-4-6") { return "Claude Opus 4.6" } else if strings.Contains(ml, "opus") { return "Claude Opus 4.5" } else if strings.Contains(ml, "sonnet") { return "Claude Sonnet" } else if strings.Contains(ml, "haiku") { return "Claude Haiku" } else if strings.Contains(ml, "gpt-5") { return "GPT-5" } else if strings.Contains(ml, "gpt-4o") { return "GPT-4o" } else if strings.Contains(ml, "gpt-4") { return "GPT-4" } else if strings.Contains(ml, "gemini") { return "Gemini" }; return model
}

func (dp *DataProvider) GetDetailedUsage() (map[string]UsageBucket, error) {
	usage := make(map[string]UsageBucket); pattern := filepath.Join(dp.BasePath, "agents", "*", "sessions", "*.jsonl"); files, _ := filepath.Glob(pattern)
	for _, f := range files {
		file, err := os.Open(f); if err != nil { continue }; scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var line struct { Message struct { Role string `json:"role"`; Model string `json:"model"`; Usage struct { Input int64 `json:"input"`; Output int64 `json:"output"`; CacheRead int64 `json:"cacheRead"`; TotalTokens int64 `json:"totalTokens"`; Cost struct { Total float64 `json:"total"` } `json:"cost"` } `json:"usage"` } `json:"message"` }
			if err := json.Unmarshal(scanner.Bytes(), &line); err != nil { continue }; if line.Message.Role != "assistant" || line.Message.Model == "" { continue }
			name := FriendlyModelName(line.Message.Model); bucket := usage[name]; bucket.Calls++; bucket.Input += line.Message.Usage.Input; bucket.Output += line.Message.Usage.Output; bucket.CacheRead += line.Message.Usage.CacheRead; bucket.TotalTokens += line.Message.Usage.TotalTokens; bucket.Cost += line.Message.Usage.Cost.Total; usage[name] = bucket
		}
		file.Close()
	}
	return usage, nil
}

func (dp *DataProvider) GetGeneratedAlerts() ([]Alert, error) {
	var alerts []Alert; stats, err := exec.Command("ps", "-A", "-o", "rss=").Output()
	if err == nil { totalRSS := 0.0; for _, line := range strings.Fields(string(stats)) { rss, _ := strconv.ParseFloat(line, 64); totalRSS += rss }; if totalRSS > 512000 { alerts = append(alerts, Alert{Level: "WARNING", Message: fmt.Sprintf("High system memory usage: %.0f MB", totalRSS/1024), Time: time.Now().Format("15:04:05")}) } }
	usage, _ := dp.GetDetailedUsage(); totalCost := 0.0; for _, b := range usage { totalCost += b.Cost }; if totalCost > 20.0 { alerts = append(alerts, Alert{Level: "WARNING", Message: fmt.Sprintf("Daily cost exceeds $20: $%.2f", totalCost), Time: time.Now().Format("15:04:05")}) }
	return alerts, nil
}
