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
	"time"
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
	ID        string `json:"id"`
	Name      string `json:"name"`
	Workspace string `json:"workspace"`
	AgentDir  string `json:"agent_dir"`
	Sessions  string `json:"sessions_dir"`
	IsDefault bool   `json:"is_default"`
}

type DataProvider struct {
	BasePath string
}

func NewDataProvider() (*DataProvider, error) {
	basePath := os.Getenv("OPENCLAW_STATE_DIR")
	if basePath == "" {
		basePath = os.Getenv("CLAWDBOT_STATE_DIR")
	}

	if basePath == "" {
		// Priority search list for state directory
		candidates := []string{
			".openclaw",
			"/home/jason9075/.openclaw",
			"/home/clawbot/.openclaw",
		}

		// Search all /home/* directories
		entries, err := os.ReadDir("/home")
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					candidates = append(candidates, filepath.Join("/home", e.Name(), ".openclaw"))
				}
			}
		}

		// Find first existing that contains openclaw.json or subagents.json
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

		// Fallback to current user home
		if basePath == "" {
			if home, err := os.UserHomeDir(); err == nil {
				basePath = filepath.Join(home, ".openclaw")
			}
		}
	}

	// Ensure directory exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		_ = os.MkdirAll(basePath, 0755)
	}

	fmt.Fprintf(os.Stderr, "DataProvider initialized with BasePath: %s\n", basePath)

	return &DataProvider{BasePath: basePath}, nil
}

func (dp *DataProvider) GetAgentPersonas() ([]AgentPersona, error) {
	configPath := filepath.Join(dp.BasePath, "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Fallback to minimal "main" agent if config missing
		return []AgentPersona{
			{
				ID:        "main",
				Name:      "Main Agent",
				Workspace: filepath.Join(dp.BasePath, "workspace"),
				AgentDir:  filepath.Join(dp.BasePath, "agents", "main", "agent"),
				Sessions:  filepath.Join(dp.BasePath, "agents", "main", "sessions"),
				IsDefault: true,
			},
		}, nil
	}

	var cfg struct {
		Agents struct {
			List []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Workspace string `json:"workspace"`
				AgentDir  string `json:"agentDir"`
				Default   bool   `json:"default"`
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

		workspace := a.Workspace
		if workspace == "" {
			if id == "main" {
				workspace = filepath.Join(dp.BasePath, "workspace")
			} else {
				workspace = filepath.Join(dp.BasePath, "workspace-"+id)
			}
		}

		agentDir := a.AgentDir
		if agentDir == "" {
			agentDir = filepath.Join(dp.BasePath, "agents", id, "agent")
		}

		sessionsDir := filepath.Join(dp.BasePath, "agents", id, "sessions")

		name := a.Name
		if name == "" {
			name = strings.Title(id)
		}

		personas = append(personas, AgentPersona{
			ID:        id,
			Name:      name,
			Workspace: workspace,
			AgentDir:  agentDir,
			Sessions:  sessionsDir,
			IsDefault: a.Default || (id == "main" && len(cfg.Agents.List) == 1),
		})
	}

	if len(personas) == 0 {
		personas = append(personas, AgentPersona{
			ID:        "main",
			Name:      "Main Agent",
			Workspace: filepath.Join(dp.BasePath, "workspace"),
			AgentDir:  filepath.Join(dp.BasePath, "agents", "main", "agent"),
			Sessions:  filepath.Join(dp.BasePath, "agents", "main", "sessions"),
			IsDefault: true,
		})
	}

	return personas, nil
}

func (dp *DataProvider) GetSubAgentActivity() ([]SubAgentRun, error) {
	var allRuns []SubAgentRun

	// 1. Try legacy/current format: subagents.json
	legacyPath := filepath.Join(dp.BasePath, "subagents.json")
	if data, err := os.ReadFile(legacyPath); err == nil {
		var runs []SubAgentRun
		if err := json.Unmarshal(data, &runs); err == nil {
			allRuns = append(allRuns, runs...)
		}
	}

	// 2. Try new format: subagents/runs.json
	path := filepath.Join(dp.BasePath, "subagents", "runs.json")
	if data, err := os.ReadFile(path); err == nil {
		var registry PersistedSubagentRegistry
		if err := json.Unmarshal(data, &registry); err == nil {
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
		json.Unmarshal(data, &models)
		return models, nil
	}
	return []Model{}, nil
}

type UsageBucket struct {
	Calls       int     `json:"calls"`
	Input       int64   `json:"input"`
	Output      int64   `json:"output"`
	CacheRead   int64   `json:"cache_read"`
	TotalTokens int64   `json:"total_tokens"`
	Cost        float64 `json:"cost"`
}

type ModelUsage struct {
	Model string `json:"model"`
	UsageBucket
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

	// 1. Memory usage alert
	stats, err := exec.Command("ps", "-A", "-o", "rss=").Output()
	if err == nil {
		totalRSS := 0.0
		for _, line := range strings.Fields(string(stats)) {
			rss, _ := strconv.ParseFloat(line, 64)
			totalRSS += rss
		}
		if totalRSS > 512000 { // > 500MB
			alerts = append(alerts, Alert{
				Level:   "WARNING",
				Message: fmt.Sprintf("High system memory usage: %.0f MB", totalRSS/1024),
				Time:    time.Now().Format("15:04:05"),
			})
		}
	}

	// 2. Cost alert
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
