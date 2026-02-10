package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

func (dp *DataProvider) GetSubAgentActivity() ([]SubAgentRun, error) {
	var allRuns []SubAgentRun

	// 1. Try multiple possible paths for subagents.json across users if needed
	paths := []string{
		filepath.Join(dp.BasePath, "subagents.json"),
		"/home/jason9075/.openclaw/subagents.json",
		"/home/clawbot/.openclaw/subagents.json",
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			var runs []SubAgentRun
			if err := json.Unmarshal(data, &runs); err == nil {
				allRuns = append(allRuns, runs...)
				break // Found one, good enough
			}
		}
	}

	// 2. Try new format: subagents/runs.json
	registryPath := filepath.Join(dp.BasePath, "subagents", "runs.json")
	if data, err := os.ReadFile(registryPath); err == nil {
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

func (dp *DataProvider) GetDefinedAgents() ([]SubAgentRun, error) {
	var agents []SubAgentRun

	// 1. Try reading from openclaw.json
	configPath := filepath.Join(dp.BasePath, "openclaw.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var cfg struct {
			Agents struct {
				List []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"list"`
			} `json:"agents"`
		}
		if err := json.Unmarshal(data, &cfg); err == nil {
			for _, a := range cfg.Agents.List {
				name := a.Name
				if name == "" {
					name = a.ID
				}
				agents = append(agents, SubAgentRun{
					ID:     a.ID,
					Name:   name,
					Status: "online",
				})
			}
		}
	}

	// 2. Scan agents/ directory
	agentsDir := filepath.Join(dp.BasePath, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				id := e.Name()
				// Avoid duplicates
				exists := false
				for _, a := range agents {
					if a.ID == id {
						exists = true
						break
					}
				}
				if !exists && id != "main" {
					agents = append(agents, SubAgentRun{
						ID:     id,
						Name:   strings.Title(id),
						Status: "online",
					})
				}
			}
		}
	}

	return agents, nil
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
	data, err := os.ReadFile(path)
	if err == nil {
		var todos []TodoItem
		json.Unmarshal(data, &todos)
		return todos, nil
	}
	return []TodoItem{}, nil
}

func (dp *DataProvider) GetActiveSessions() (map[string]Session, error) {
	path := filepath.Join(dp.BasePath, "sessions", "active.json")
	data, err := os.ReadFile(path)
	if err == nil {
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

func (dp *DataProvider) GetSkills() ([]Skill, error) {
	path := filepath.Join(dp.BasePath, "skills.json")
	if data, err := os.ReadFile(path); err == nil {
		var skills []Skill
		json.Unmarshal(data, &skills)
		return skills, nil
	}
	return []Skill{}, nil
}
