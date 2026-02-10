package data

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type TodoItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"` // "todo", "doing", "done"
	Priority  string `json:"priority"`
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
	Model       string  `json:"model"`
	InputTokens int     `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	TotalCost   float64 `json:"total_cost"`
}

type SubAgentRun struct {
	Name     string  `json:"name"`
	status   string  `json:"status"` // "running", "completed", "failed"
	Duration string  `json:"duration"`
	Cost     float64 `json:"cost"`
	Tokens   int     `json:"tokens"`
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
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	basePath := filepath.Join(home, ".openclaw")
	
	// Ensure directory exists
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		_ = os.MkdirAll(basePath, 0755)
	}

	return &DataProvider{BasePath: basePath}, nil
}

func (dp *DataProvider) GetTodos() ([]TodoItem, error) {
	path := filepath.Join(dp.BasePath, "todo.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []TodoItem{}, nil
		}
		return nil, err
	}

	var todos []TodoItem
	if err := json.Unmarshal(data, &todos); err != nil {
		return nil, err
	}
	return todos, nil
}

func (dp *DataProvider) GetActiveSessions() (map[string]Session, error) {
	path := filepath.Join(dp.BasePath, "sessions", "active.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Session{}, nil
		}
		return nil, err
	}

	var sessions map[string]Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, err
	}
	return sessions, nil
}

func (dp *DataProvider) GetAlerts() ([]Alert, error) {
	path := filepath.Join(dp.BasePath, "logs", "error.log")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Alert{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var alerts []Alert
	scanner := bufio.NewScanner(file)
	// Read last 10 lines - simplistic approach
	// In a real scenario we'd seek to end and read backwards, 
	// but for now reading all and taking last 5 is fine for small logs.
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	start := 0
	if len(lines) > 5 {
		start = len(lines) - 5
	}

	for _, line := range lines[start:] {
		// Simple parser: [LEVEL] Time - Message
		// Example: [ERROR] 2023-10-01 12:00:00 - Something failed
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
		} else {
            // Fallback for unstructured logs
            alerts = append(alerts, Alert{
                Level: "INFO",
                Message: line,
                Time: "Just now",
            })
        }
	}

	return alerts, nil
}

func (dp *DataProvider) GetGitLog() ([]GitCommit, error) {
	// Execute git log in the current working directory
	// Assumes the binary is running in a git repo or CWD is set correctly
	cmd := exec.Command("git", "log", "-n", "5", "--oneline")
	output, err := cmd.Output()
	if err != nil {
		return []GitCommit{}, err
	}

	var commits []GitCommit
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			commits = append(commits, GitCommit{
				Hash:    parts[0],
				Message: parts[1],
			})
		}
	}
	return commits, nil
}

func (dp *DataProvider) GetCronJobs() ([]CronJob, error) {
	path := filepath.Join(dp.BasePath, "crontabs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []CronJob{}, nil
		}
		return nil, err
	}

	var jobs []CronJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (dp *DataProvider) GetCosts() ([]CostCard, error) {
	path := filepath.Join(dp.BasePath, "costs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []CostCard{}, nil
		}
		return nil, err
	}

	var costs []CostCard
	if err := json.Unmarshal(data, &costs); err != nil {
		return nil, err
	}
	return costs, nil
}

func (dp *DataProvider) GetTokenUsage() ([]TokenStats, error) {
	path := filepath.Join(dp.BasePath, "tokens.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []TokenStats{}, nil
		}
		return nil, err
	}

	var stats []TokenStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func (dp *DataProvider) GetSubAgentActivity() ([]SubAgentRun, error) {
	path := filepath.Join(dp.BasePath, "subagents.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []SubAgentRun{}, nil
		}
		return nil, err
	}

	var runs []SubAgentRun
	if err := json.Unmarshal(data, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func (dp *DataProvider) GetModels() ([]Model, error) {
	path := filepath.Join(dp.BasePath, "models.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Model{}, nil
		}
		return nil, err
	}

	var models []Model
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, err
	}
	return models, nil
}

func (dp *DataProvider) GetSkills() ([]Skill, error) {
	path := filepath.Join(dp.BasePath, "skills.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Skill{}, nil
		}
		return nil, err
	}

	var skills []Skill
	if err := json.Unmarshal(data, &skills); err != nil {
		return nil, err
	}
	return skills, nil
}
