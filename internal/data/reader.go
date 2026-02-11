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
	Status   string `json:"status"` // "todo", "doing", "done"
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
	Schedule string `json:"schedule"`
	Command  string `json:"command"`
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

// SubAgentRun represents the data sent to the frontend
type SubAgentRun struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"` // "running", "completed", "failed", "online"
	Duration string  `json:"duration"`
	Cost     float64 `json:"cost"`
	Tokens   int     `json:"tokens"`
	ID       string  `json:"id,omitempty"`
}

// Internal structs for parsing OpenClaw registry
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
	Type string `json:"type"` // "chat", "embedding"
}

type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AgentPersona represents a long-lived agent configuration
type AgentPersona struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Emoji     string   `json:"emoji"`
	Workspace string   `json:"workspace"`
	AgentDir  string   `json:"agent_dir"`
	Sessions  string   `json:"sessions_dir"`
	IsDefault bool     `json:"is_default"`
	Skills    []string `json:"skills"` // Only non-built-in skills
	Status    string   `json:"status"` // "idle", "thinking"
}

type ConversationTurn struct {
	UserMessage  string  `json:"user_message"`
	Reasoning    string  `json:"reasoning"`
	FinalText    string  `json:"final_text"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

type SessionDetails struct {
	AgentID   string             `json:"agent_id"`
	SessionID string             `json:"session_id"`
	Turns     []ConversationTurn `json:"turns"`
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
	if basePath == "" {
		basePath = os.Getenv("CLAWDBOT_STATE_DIR")
	}

	if basePath == "" {
		candidates := []string{
			".openclaw",
			"/home/jason9075/.openclaw",
			"/home/clawbot/.openclaw",
		}

		entries, err := os.ReadDir("/home")
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					candidates = append(candidates, filepath.Join("/home", e.Name(), ".openclaw"))
				}
			}
		}

		for _, c := range candidates {
			if _, err := os.Stat(filepath.Join(c, "openclaw.json")); err == nil {
				basePath = c
				break
			}
			if _, err := os.Stat(filepath.Join(c, "subagents.json")); err == nil {
				basePath = c
				break
			}
		}

		if basePath == "" {
			if home, err := os.UserHomeDir(); err == nil {
				basePath = filepath.Join(home, ".openclaw")
			}
		}
	}

	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		_ = os.MkdirAll(basePath, 0755)
	}

	dp := &DataProvider{
		BasePath:       basePath,
		sessionToAgent: make(map[string]string),
		agentStatus:    make(map[string]string),
		agentReasoning: make(map[string]string),
		lastUpdate:     make(map[string]time.Time),
	}

	fmt.Fprintf(os.Stderr, "DataProvider initialized with BasePath: %s\n", basePath)

	dp.RefreshSessionMap()

	return dp, nil
}

func (dp *DataProvider) expandPath(p string) string {
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}

func (dp *DataProvider) normalizeOpenClawPath(p string) string {
	p = dp.expandPath(p)
	if strings.Contains(p, ".openclaw") {
		parts := strings.SplitN(p, ".openclaw", 2)
		if len(parts) == 2 {
			newPath := filepath.Join(dp.BasePath, parts[1])
			if _, err := os.Stat(newPath); err == nil {
				return newPath
			}
		}
	}
	return p
}

func (dp *DataProvider) RefreshSessionMap() {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	dp.sessionToAgent = make(map[string]string)

	agentsDir := filepath.Join(dp.BasePath, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return
	}

	for _, agentEntry := range entries {
		if !agentEntry.IsDir() {
			continue
		}
		agentID := agentEntry.Name()
		sessionsDir := filepath.Join(agentsDir, agentID, "sessions")
		
		sessions, err := os.ReadDir(sessionsDir)
		if err != nil {
			continue
		}

		for _, s := range sessions {
			if !s.IsDir() && filepath.Ext(s.Name()) == ".jsonl" {
				sessionID := strings.TrimSuffix(s.Name(), ".jsonl")
				dp.sessionToAgent[sessionID] = agentID
			}
		}
	}
}

func (dp *DataProvider) WatchRawStream(broadcaster *Broadcaster) {
	logPath := filepath.Join(dp.BasePath, "logs", "raw-stream.jsonl")
	os.MkdirAll(filepath.Dir(logPath), 0755)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create raw stream watcher: %v\n", err)
		return
	}

	watcher.Add(filepath.Dir(logPath))

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		for range ticker.C {
			dp.mu.Lock()
			now := time.Now()
			for agentID, last := range dp.lastUpdate {
				if dp.agentStatus[agentID] == "thinking" && now.Sub(last) > 3*time.Second {
					dp.agentStatus[agentID] = "idle"
					dp.agentReasoning[agentID] = ""
					broadcaster.Broadcast(Event{
						Type: "agent_status",
						Payload: map[string]string{
							"agent_id": agentID,
							"status":   "idle",
						},
					})
				}
			}
			dp.mu.Unlock()
		}
	}()

	go func() {
		defer watcher.Close()
		var lastSize int64
		if info, err := os.Stat(logPath); err == nil {
			lastSize = info.Size()
		}

		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Name == logPath && (event.Op&fsnotify.Write == fsnotify.Write) {
					info, err := os.Stat(logPath)
					if err != nil {
						continue
					}
					if info.Size() > lastSize {
						dp.processRawStreamNewData(logPath, lastSize, broadcaster)
						lastSize = info.Size()
					} else if info.Size() < lastSize {
						lastSize = info.Size()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "raw stream watcher error: %v\n", err)
			}
		}
	}()
}

func (dp *DataProvider) processRawStreamNewData(path string, offset int64, broadcaster *Broadcaster) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	file.Seek(offset, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		var entry struct {
			Event     string `json:"event"`
			SessionID string `json:"sessionId"`
			Delta     string `json:"delta"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}

		if entry.SessionID == "" {
			continue
		}

		dp.mu.RLock()
		agentID, ok := dp.sessionToAgent[entry.SessionID]
		dp.mu.RUnlock()

		if !ok {
			dp.RefreshSessionMap()
			dp.mu.RLock()
			agentID = dp.sessionToAgent[entry.SessionID]
			dp.mu.RUnlock()
		}

		if agentID != "" {
			dp.mu.Lock()
			dp.agentStatus[agentID] = "thinking"
			dp.lastUpdate[agentID] = time.Now()
			
			if entry.Delta != "" {
				dp.agentReasoning[agentID] += entry.Delta
				broadcaster.Broadcast(Event{
					Type: "reasoning_delta",
					Payload: map[string]string{
						"agent_id": agentID,
						"delta":    entry.Delta,
					},
				})
			}
			dp.mu.Unlock()

			broadcaster.Broadcast(Event{
				Type: "agent_status",
				Payload: map[string]string{
					"agent_id": agentID,
					"status":   "thinking",
				},
			})
		}
	}
}

func (dp *DataProvider) GetSessionsForAgent(agentID string) ([]SessionInfo, error) {
	sessionsDir := filepath.Join(dp.BasePath, "agents", agentID, "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".jsonl" {
			info, err := e.Info()
			if err != nil {
				continue
			}
			sessions = append(sessions, SessionInfo{
				ID:        strings.TrimSuffix(e.Name(), ".jsonl"),
				UpdatedAt: info.ModTime(),
			})
		}
	}
	return sessions, nil
}

func (dp *DataProvider) GetSessionDetails(agentID string, sessionID string) (*SessionDetails, error) {
	sessionFile := filepath.Join(dp.BasePath, "agents", agentID, "sessions", sessionID+".jsonl")
	file, err := os.Open(sessionFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	details := &SessionDetails{
		AgentID:   agentID,
		SessionID: sessionID,
		Turns:     []ConversationTurn{},
	}

	scanner := bufio.NewScanner(file)
	var currentTurn *ConversationTurn
	var reasoningBuilder strings.Builder

	for scanner.Scan() {
		var line struct {
			Message struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}

		if line.Message.Role == "user" {
			if currentTurn != nil {
				currentTurn.Reasoning = reasoningBuilder.String()
				details.Turns = append(details.Turns, *currentTurn)
				reasoningBuilder.Reset()
			}
			currentTurn = &ConversationTurn{}

			if content, ok := line.Message.Content.(string); ok {
				currentTurn.UserMessage = content
			} else if contentArr, ok := line.Message.Content.([]interface{}); ok {
				for _, part := range contentArr {
					if p, ok := part.(map[string]interface{}); ok {
						if p["type"] == "text" {
							currentTurn.UserMessage = p["text"].(string)
						}
					}
				}
			}
		} else if line.Message.Role == "assistant" && currentTurn != nil {
			// Extract usage
			var usage struct {
				Usage struct {
					Input  int64 `json:"input"`
					Output int64 `json:"output"`
					Cost   struct {
						Total float64 `json:"total"`
					} `json:"cost"`
				} `json:"usage"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &usage); err == nil {
				currentTurn.InputTokens += usage.Usage.Input
				currentTurn.OutputTokens += usage.Usage.Output
				currentTurn.Cost += usage.Usage.Cost.Total
			}

			if content, ok := line.Message.Content.(string); ok {
				currentTurn.FinalText = content
				think := extractThinkingFromTaggedText(content)
				if think != "" {
					reasoningBuilder.WriteString(think)
					currentTurn.FinalText = stripThinkingTagsFromText(content)
				}
			} else if contentArr, ok := line.Message.Content.([]interface{}); ok {
				for _, part := range contentArr {
					if p, ok := part.(map[string]interface{}); ok {
						if p["type"] == "thinking" {
							reasoningBuilder.WriteString(p["thinking"].(string))
						} else if p["type"] == "text" {
							currentTurn.FinalText += p["text"].(string)
						}
					}
				}
			}
		}
	}

	if currentTurn != nil {
		currentTurn.Reasoning = reasoningBuilder.String()
		dp.mu.RLock()
		if dp.agentStatus[agentID] == "thinking" && dp.agentReasoning[agentID] != "" {
			currentTurn.Reasoning = dp.agentReasoning[agentID]
		}
		dp.mu.RUnlock()
		details.Turns = append(details.Turns, *currentTurn)
	}

	return details, nil
}

func stripThinkingTagsFromText(text string) string {
	re := []string{"<think>", "</think>", "<thinking>", "</thinking>", "<thought>", "</thought>"}
	res := text
	for _, r := range re {
		res = strings.ReplaceAll(res, r, "")
	}
	return strings.TrimSpace(res)
}

func extractThinkingFromTaggedText(text string) string {
	tags := [][]string{{"<think>", "</think>"}, {"<thinking>", "</thinking>"}, {"<thought>", "</thought>"}}
	for _, t := range tags {
		start := strings.Index(text, t[0])
		end := strings.Index(text, t[1])
		if start != -1 && end != -1 && end > start {
			return text[start+len(t[0]) : end]
		}
	}
	return ""
}

func (dp *DataProvider) GetAgentPersonas() ([]AgentPersona, error) {
	externalSkills := make(map[string]bool)
	skills, _ := dp.GetSkills()
	for _, s := range skills {
		externalSkills[s.Name] = true
	}

	configPath := filepath.Join(dp.BasePath, "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return []AgentPersona{{
			ID:        "main",
			Name:      "Main Agent",
			Emoji:     "🧠",
			Workspace: filepath.Join(dp.BasePath, "workspace"),
			AgentDir:  filepath.Join(dp.BasePath, "agents", "main", "agent"),
			Sessions:  filepath.Join(dp.BasePath, "agents", "main", "sessions"),
			IsDefault: true,
			Skills:    dp.getSkillNames(externalSkills),
			Status:    "idle",
		}}, nil
	}

	var cfg struct {
		Agents struct {
			List []struct {
				ID        string   `json:"id"`
				Name      string   `json:"name"`
				Workspace string   `json:"workspace"`
				AgentDir  string   `json:"agentDir"`
				Default   bool     `json:"default"`
				Skills    []string `json:"skills"`
				Identity  struct {
					Emoji string `json:"emoji"`
				} `json:"identity"`
			} `json:"list"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	var personas []AgentPersona
	for _, a := range cfg.Agents.List {
		id := a.ID
		if id == "" {
			continue
		}

		workspace := dp.normalizeOpenClawPath(a.Workspace)
		if workspace == "" {
			if id == "main" {
				workspace = filepath.Join(dp.BasePath, "workspace")
			} else {
				workspace = filepath.Join(dp.BasePath, "workspace-"+id)
			}
		}

		agentDir := dp.normalizeOpenClawPath(a.AgentDir)
		if agentDir == "" {
			agentDir = filepath.Join(dp.BasePath, "agents", id, "agent")
		}

		sessionsDir := filepath.Join(dp.BasePath, "agents", id, "sessions")
		sessionsDir = dp.normalizeOpenClawPath(sessionsDir)

		name := a.Name
		if name == "" {
			name = strings.Title(id)
		}

		emoji := a.Identity.Emoji
		if emoji == "" {
			emoji = dp.readEmojiFromIdentity(workspace)
		}
		if emoji == "" {
			lid := strings.ToLower(id)
			if strings.Contains(lid, "coder") {
				emoji = "💻"
			} else if strings.Contains(lid, "research") {
				emoji = "🔍"
			} else if strings.Contains(lid, "schedule") {
				emoji = "📅"
			} else {
				emoji = "🧠"
			}
		}

		var filteredSkills []string
		if a.Skills != nil {
			for _, s := range a.Skills {
				if externalSkills[s] {
					filteredSkills = append(filteredSkills, s)
				}
			}
		} else {
			filteredSkills = dp.getSkillNames(externalSkills)
		}

		dp.mu.RLock()
		status := dp.agentStatus[id]
		if status == "" {
			status = "idle"
		}
		dp.mu.RUnlock()

		personas = append(personas, AgentPersona{
			ID:        id,
			Name:      name,
			Emoji:     emoji,
			Workspace: workspace,
			AgentDir:  agentDir,
			Sessions:  sessionsDir,
			IsDefault: a.Default || (id == "main" && len(cfg.Agents.List) == 1),
			Skills:    filteredSkills,
			Status:    status,
		})
	}

	if len(personas) == 0 {
		dp.mu.RLock()
		status := dp.agentStatus["main"]
		if status == "" {
			status = "idle"
		}
		dp.mu.RUnlock()

		personas = append(personas, AgentPersona{
			ID:        "main",
			Name:      "Main Agent",
			Emoji:     "🧠",
			Workspace: filepath.Join(dp.BasePath, "workspace"),
			AgentDir:  filepath.Join(dp.BasePath, "agents", "main", "agent"),
			Sessions:  filepath.Join(dp.BasePath, "agents", "main", "sessions"),
			IsDefault: true,
			Skills:    dp.getSkillNames(externalSkills),
			Status:    status,
		})
	}

	return personas, nil
}

func (dp *DataProvider) GetAgentPersonasInternal() ([]AgentPersona, error) {
	configPath := filepath.Join(dp.BasePath, "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return []AgentPersona{{
			Workspace: filepath.Join(dp.BasePath, "workspace"),
		}}, nil
	}
	var cfg struct {
		Agents struct {
			List []struct {
				ID        string `json:"id"`
				Workspace string `json:"workspace"`
			} `json:"list"`
		} `json:"agents"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	var personas []AgentPersona
	for _, a := range cfg.Agents.List {
		workspace := dp.normalizeOpenClawPath(a.Workspace)
		if workspace == "" {
			if a.ID == "main" {
				workspace = filepath.Join(dp.BasePath, "workspace")
			} else {
				workspace = filepath.Join(dp.BasePath, "workspace-"+a.ID)
			}
		}
		personas = append(personas, AgentPersona{Workspace: workspace})
	}
	return personas, nil
}

func (dp *DataProvider) getSkillNames(m map[string]bool) []string {
	var names []string
	for k := range m {
		names = append(names, k)
	}
	return names
}

func (dp *DataProvider) readEmojiFromIdentity(workspace string) string {
	path := filepath.Join(workspace, "IDENTITY.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "Emoji:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				emoji := strings.TrimSpace(parts[1])
				emoji = strings.Trim(emoji, "*")
				emoji = strings.TrimSpace(emoji)
				if emoji != "" {
					return emoji
				}
			}
		}
	}
	return ""
}

func (dp *DataProvider) GetSubAgentActivity() ([]SubAgentRun, error) {
	var allRuns []SubAgentRun
	legacyPath := filepath.Join(dp.BasePath, "subagents.json")
	if data, err := os.ReadFile(legacyPath); err == nil {
		var runs []SubAgentRun
		if err := json.Unmarshal(data, &runs); err == nil {
			allRuns = append(allRuns, runs...)
		}
	}
	path := filepath.Join(dp.BasePath, "subagents", "runs.json")
	if data, err := os.ReadFile(path); err == nil {
		var registry PersistedSubagentRegistry
		if err := json.Unmarshal(data, &registry); err != nil {
			for _, record := range registry.Runs {
				name := record.Label
				if name == "" {
					name = record.Task
				}
				status := "running"
				if record.Outcome != nil {
					if record.Outcome.Status == "ok" {
						status = "completed"
					} else if record.Outcome.Status == "error" {
						status = "failed"
					}
				}
				duration := "--"
				if record.StartedAt > 0 {
					var d time.Duration
					if record.EndedAt > 0 {
						d = time.Duration(record.EndedAt-record.StartedAt) * time.Millisecond
					} else {
						d = time.Since(time.Unix(0, record.StartedAt*int64(time.Millisecond)))
					}
					duration = d.Round(time.Second).String()
				}
				allRuns = append(allRuns, SubAgentRun{
					ID:       record.RunID,
					Name:     name,
					Status:   status,
					Duration: duration,
				})
			}
		}
	}
	return allRuns, nil
}

func (dp *DataProvider) WatchTranscripts(broadcaster *Broadcaster) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	personas, _ := dp.GetAgentPersonas()
	for _, p := range personas {
		if _, err := os.Stat(p.Sessions); err == nil {
			watcher.Add(p.Sessions)
		}
	}
	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					if filepath.Ext(event.Name) == ".jsonl" {
						dp.handleTranscriptChange(event.Name, broadcaster)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "watcher error: %v\n", err)
			}
		}
	}()
	return nil
}

func (dp *DataProvider) handleTranscriptChange(path string, broadcaster *Broadcaster) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	stat, _ := file.Stat()
	if stat.Size() < 2000 {
		file.Seek(0, 0)
	} else {
		file.Seek(-2000, 2)
	}
	scanner := bufio.NewScanner(file)
	var lastLine string
	for scanner.Scan() {
		lastLine = scanner.Text()
	}
	if lastLine == "" {
		return
	}
	var entry struct {
		Message struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Name string `json:"name"`
			} `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}
	if err := json.Unmarshal([]byte(lastLine), &entry); err != nil {
		return
	}
	agentID := filepath.Base(filepath.Dir(filepath.Dir(path)))
	for _, c := range entry.Message.Content {
		if c.Type == "tool_use" || c.Type == "tool_call" {
			broadcaster.Broadcast(Event{
				Type: "skill_use",
				Payload: map[string]string{
					"agent_id": agentID,
					"skill":    c.Name,
					"status":   "executing",
				},
			})
			return
		}
	}
	for _, tc := range entry.Message.ToolCalls {
		if tc.Function.Name != "" {
			broadcaster.Broadcast(Event{
				Type: "skill_use",
				Payload: map[string]string{
					"agent_id": agentID,
					"skill":    tc.Function.Name,
					"status":   "executing",
				},
			})
			return
		}
	}
	broadcaster.Broadcast(Event{
		Type:    "refresh",
		Payload: map[string]string{"reason": "transcript_update", "agent_id": agentID},
	})
}

func (dp *DataProvider) ListFiles() ([]string, error) {
	var files []string
	filepath.Walk(dp.BasePath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(dp.BasePath, path)
			files = append(files, rel)
		}
		return nil
	})
	return files, nil
}

func (dp *DataProvider) GetTodos() ([]TodoItem, error) {
	path := filepath.Join(dp.BasePath, "todo.json")
	if data, err := os.ReadFile(path); err == nil {
		var todos []TodoItem
		json.Unmarshal(data, &todos)
		return todos, nil
	}
	return []TodoItem{}, nil
}

func (dp *DataProvider) GetActiveSessions() (map[string]Session, error) {
	path := filepath.Join(dp.BasePath, "sessions", "active.json")
	if data, err := os.ReadFile(path); err == nil {
		var sessions map[string]Session
		json.Unmarshal(data, &sessions)
		return sessions, nil
	}
	return map[string]Session{}, nil
}

func (dp *DataProvider) GetAlerts() ([]Alert, error) {
	path := filepath.Join(dp.BasePath, "logs", "error.log")
	file, err := os.Open(path)
	if err != nil {
		return []Alert{}, nil
	}
	defer file.Close()
	var alerts []Alert
	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	start := 0
	if len(lines) > 5 {
		start = len(lines) - 5
	}
	for _, line := range lines[start:] {
		parts := strings.SplitN(line, " - ", 2)
		if len(parts) == 2 {
			meta := strings.Fields(parts[0])
			if len(meta) >= 2 {
				alerts = append(alerts, Alert{
					Level:   strings.Trim(meta[0], "[]"),
					Time:    strings.Join(meta[1:], " "),
					Message: parts[1],
				})
			}
		}
	}
	return alerts, nil
}

func (dp *DataProvider) GetGitLog() ([]GitCommit, error) {
	cmd := exec.Command("git", "log", "-n", "5", "--oneline")
	output, err := cmd.Output()
	if err != nil {
		return []GitCommit{}, nil
	}
	var commits []GitCommit
	for _, line := range strings.Split(string(output), "\n") {
		if parts := strings.SplitN(line, " ", 2); len(parts) == 2 {
			commits = append(commits, GitCommit{Hash: parts[0], Message: parts[1]})
		}
	}
	return commits, nil
}

func (dp *DataProvider) GetCronJobs() ([]CronJob, error) {
	path := filepath.Join(dp.BasePath, "crontabs.json")
	if data, err := os.ReadFile(path); err == nil {
		var jobs []CronJob
		json.Unmarshal(data, &jobs)
		return jobs, nil
	}
	return []CronJob{}, nil
}

func (dp *DataProvider) GetCosts() ([]CostCard, error) {
	path := filepath.Join(dp.BasePath, "costs.json")
	if data, err := os.ReadFile(path); err == nil {
		var costs []CostCard
		json.Unmarshal(data, &costs)
		return costs, nil
	}
	return []CostCard{}, nil
}

func (dp *DataProvider) GetTokenUsage() ([]TokenStats, error) {
	path := filepath.Join(dp.BasePath, "tokens.json")
	if data, err := os.ReadFile(path); err == nil {
		var stats []TokenStats
		json.Unmarshal(data, &stats)
		return stats, nil
	}
	return []TokenStats{}, nil
}

func (dp *DataProvider) GetModels() ([]Model, error) {
	path := filepath.Join(dp.BasePath, "models.json")
	if data, err := os.ReadFile(path); err == nil {
		var models []Model
		if err := json.Unmarshal(data, &models); err != nil {
			return models, nil
		}
	}
	configPath := filepath.Join(dp.BasePath, "openclaw.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			Agents struct {
				Defaults struct {
					Models map[string]interface{} `json:"models"`
				} `json:"defaults"`
			} `json:"agents"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			var models []Model
			for id := range cfg.Agents.Defaults.Models {
				models = append(models, Model{
					Name: id,
					Type: "chat",
				})
			}
			return models, nil
		}
	}
	return []Model{}, nil
}

func (dp *DataProvider) GetSkills() ([]Skill, error) {
	var allSkills []Skill
	seen := make(map[string]bool)
	configPath := filepath.Join(dp.BasePath, "openclaw.json")
	var extraDirs []string
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			Skills struct {
				Load struct {
					ExtraDirs []string `json:"extraDirs"`
				} `json:"load"`
			} `json:"skills"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			extraDirs = cfg.Skills.Load.ExtraDirs
		}
	}
	managedPath := filepath.Join(dp.BasePath, "skills")
	dp.scanSkillsDir(managedPath, &allSkills, seen)
	for _, dir := range extraDirs {
		dp.scanSkillsDir(dp.normalizeOpenClawPath(dir), &allSkills, seen)
	}
	personas, _ := dp.GetAgentPersonasInternal()
	for _, p := range personas {
		workspaceSkills := filepath.Join(p.Workspace, "skills")
		dp.scanSkillsDir(workspaceSkills, &allSkills, seen)
	}
	return allSkills, nil
}

func (dp *DataProvider) scanSkillsDir(dir string, skills *[]Skill, seen map[string]bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillID := e.Name()
		if seen[skillID] {
			continue
		}
		skillPath := filepath.Join(dir, skillID, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			file, err := os.Open(skillPath)
			if err != nil {
				continue
			}
			
			name := skillID
			description := ""
			scanner := bufio.NewScanner(file)
			inFrontmatter := false
			for scanner.Scan() {
				line := scanner.Text()
				if line == "---" {
					if !inFrontmatter {
						inFrontmatter = true
						continue
					} else {
						break
					}
				}
				if inFrontmatter {
					if strings.HasPrefix(line, "name:") {
						name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
					} else if strings.HasPrefix(line, "description:") {
						description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
						description = strings.Trim(description, "\"")
					}
				}
			}
			file.Close()
			*skills = append(*skills, Skill{
				Name:        name,
				Description: description,
			})
			seen[skillID] = true
		}
	}
}

type UsageBucket struct {
	Calls       int     `json:"calls"`
	Input       int64   `json:"input"`
	Output      int64   `json:"output"`
	CacheRead   int64   `json:"cache_read"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
}

func FriendlyModelName(model string) string {
	ml := strings.ToLower(model)
	if strings.Contains(ml, "opus-4-6") {
		return "Claude Opus 4.6"
	} else if strings.Contains(ml, "opus") {
		return "Claude Opus 4.5"
	} else if strings.Contains(ml, "sonnet") {
		return "Claude Sonnet"
	} else if strings.Contains(ml, "haiku") {
		return "Claude Haiku"
	} else if strings.Contains(ml, "gpt-5") {
		return "GPT-5"
	} else if strings.Contains(ml, "gpt-4o") {
		return "GPT-4o"
	} else if strings.Contains(ml, "gpt-4") {
		return "GPT-4"
	} else if strings.Contains(ml, "gemini") {
		return "Gemini"
	}
	return model
}

func (dp *DataProvider) GetDetailedUsage() (map[string]UsageBucket, error) {
	usage := make(map[string]UsageBucket)
	pattern := filepath.Join(dp.BasePath, "agents", "*", "sessions", "*.jsonl")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		file, err := os.Open(f)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var line struct {
				Message struct {
					Role  string `json:"role"`
					Model string `json:"model"`
					Usage struct {
						Input       int64 `json:"input"`
						Output      int64 `json:"output"`
						CacheRead   int64 `json:"cacheRead"`
						TotalTokens int64 `json:"totalTokens"`
						Cost        struct {
							Total float64 `json:"total"`
						} `json:"cost"`
					} `json:"usage"`
				} `json:"message"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
				continue
			}
			if line.Message.Role != "assistant" || line.Message.Model == "" {
				continue
			}
			name := FriendlyModelName(line.Message.Model)
			bucket := usage[name]
			bucket.Calls++
			bucket.Input += line.Message.Usage.Input
			bucket.Output += line.Message.Usage.Output
			bucket.CacheRead += line.Message.Usage.CacheRead
			bucket.TotalTokens += line.Message.Usage.TotalTokens
			bucket.Cost += line.Message.Usage.Cost.Total
			usage[name] = bucket
		}
		file.Close()
	}
	return usage, nil
}

func (dp *DataProvider) GetGeneratedAlerts() ([]Alert, error) {
	var alerts []Alert
	stats, err := exec.Command("ps", "-A", "-o", "rss=").Output()
	if err == nil {
		totalRSS := 0.0
		for _, line := range strings.Fields(string(stats)) {
			rss, _ := strconv.ParseFloat(line, 64)
			totalRSS += rss
		}
		if totalRSS > 512000 {
			alerts = append(alerts, Alert{
				Level:   "WARNING",
				Message: fmt.Sprintf("High system memory usage: %.0f MB", totalRSS/1024),
				Time:    time.Now().Format("15:04:05"),
			})
		}
	}
	usage, err := dp.GetDetailedUsage()
	if err == nil {
		totalCost := 0.0
		for _, b := range usage {
			totalCost += b.Cost
		}
		if totalCost > 20.0 {
			alerts = append(alerts, Alert{
				Level:   "WARNING",
				Message: fmt.Sprintf("Daily cost exceeds $20: $%.2f", totalCost),
				Time:    time.Now().Format("15:04:05"),
			})
		}
	}
	return alerts, nil
}
