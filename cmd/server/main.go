package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"openclaw/internal/data"
	"openclaw/internal/monitor"
	"openclaw/internal/ui"
)

type DashboardState struct {
	Timestamp     time.Time                    `json:"timestamp"`
	Uptime        string                       `json:"uptime"`
	System        monitor.SystemStats          `json:"system"`
	Todos         []data.TodoItem              `json:"todos"`
	Sessions      map[string]data.Session      `json:"sessions"`
	Alerts        []data.Alert                 `json:"alerts"`
	GitLog        []data.GitCommit             `json:"git_log"`
	CronJobs      []data.CronJob               `json:"cron_jobs"`
	Costs         []data.CostCard              `json:"costs"`
	TokenUsage    []data.TokenStats            `json:"token_usage"`
	DetailedUsage map[string]data.UsageBucket `json:"detailed_usage"`
	SubAgents     []data.SubAgentRun           `json:"sub_agents"`
	Personas      []data.AgentPersona          `json:"personas"`
	Models        []data.Model                 `json:"models"`
	Skills        []data.Skill                 `json:"skills"`
	BasePath      string                       `json:"base_path"`
}

var startTime = time.Now()

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize Data Provider
	dp, err := data.NewDataProvider()
	if err != nil {
		logger.Error("failed to initialize data provider", "error", err)
	}

	// Initialize Broadcaster for real-time events
	broadcaster := data.NewBroadcaster()
	go broadcaster.Run()

	// Start Watchers
	if dp != nil {
		if err := dp.WatchTranscripts(broadcaster); err != nil {
			logger.Warn("failed to start transcript watcher", "error", err)
		}
		dp.WatchRawStream(broadcaster)
	}

	// Setup UI file server
	uiFS, err := ui.GetAssets()
	if err != nil {
		logger.Error("failed to create UI file system", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(uiFS)))
	mux.HandleFunc("/agents", handleAgents(uiFS))
	mux.HandleFunc("/api/status", handleStatus(logger, dp))
	mux.HandleFunc("/api/events", handleEvents(broadcaster))
	mux.HandleFunc("/api/hooks/receive", handleHookReceive(logger, broadcaster, dp))
	mux.HandleFunc("/api/session/list", handleSessionList(logger, dp))
	mux.HandleFunc("/api/session/details", handleSessionDetails(logger, dp))

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Server run context
	go func() {
		logger.Info("starting server", "addr", "http://localhost:"+port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	logger.Info("server exited properly")
}

func handleStatus(logger *slog.Logger, dp *data.DataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sysStats, err := monitor.GetSystemStats()
		if err != nil {
			logger.Error("failed to get system stats", "error", err)
		}

		var todos []data.TodoItem
		var sessions map[string]data.Session
		var alerts []data.Alert
		var gitLog []data.GitCommit
		var cronJobs []data.CronJob
		var costs []data.CostCard
		var tokenUsage []data.TokenStats
		var detailedUsage map[string]data.UsageBucket
		var subAgents []data.SubAgentRun
		var personas []data.AgentPersona
		var models []data.Model
		var skills []data.Skill
		var basePath string

		if dp != nil {
			todos, _ = dp.GetTodos()
			sessions, _ = dp.GetActiveSessions()
			alerts, _ = dp.GetAlerts()
			genAlerts, _ := dp.GetGeneratedAlerts()
			alerts = append(alerts, genAlerts...)
			gitLog, _ = dp.GetGitLog()
			cronJobs, _ = dp.GetCronJobs()
			costs, _ = dp.GetCosts()
			tokenUsage, _ = dp.GetTokenUsage()
			detailedUsage, _ = dp.GetDetailedUsage()
			subAgents, _ = dp.GetSubAgentActivity()
			personas, _ = dp.GetAgentPersonas()
			models, _ = dp.GetModels()
			skills, _ = dp.GetSkills()
			basePath = dp.BasePath
		}

		state := DashboardState{
			Timestamp:     time.Now(),
			Uptime:        time.Since(startTime).String(),
			System:        sysStats,
			Todos:         todos,
			Sessions:      sessions,
			Alerts:        alerts,
			GitLog:        gitLog,
			CronJobs:      cronJobs,
			Costs:         costs,
			TokenUsage:    tokenUsage,
			DetailedUsage: detailedUsage,
			SubAgents:     subAgents,
			Personas:      personas,
			Models:        models,
			Skills:        skills,
			BasePath:      basePath,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(state); err != nil {
			logger.Error("failed to encode state", "error", err)
		}
	}
}

func handleAgents(uiFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := http.FS(uiFS).Open("agents.html")
		if err != nil {
			http.Error(w, "Page not found", http.StatusNotFound)
			return
		}
		defer f.Close()
		stat, _ := f.Stat()
		http.ServeContent(w, r, "agents.html", stat.ModTime(), f)
	}
}

func handleEvents(b *data.Broadcaster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		clientChan := b.Register()
		defer b.Unregister(clientChan)

		notify := r.Context().Done()

		for {
			select {
			case <-notify:
				return
			case event := <-clientChan:
				data, _ := json.Marshal(event)
				fmt.Fprintf(w, "data: %s\n\n", string(data))
				w.(http.Flusher).Flush()
			}
		}
	}
}

func handleHookReceive(logger *slog.Logger, b *data.Broadcaster, dp *data.DataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Event      string `json:"event"`
			SessionKey string `json:"sessionKey"`
			Action     string `json:"action"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			logger.Warn("failed to decode hook payload", "error", err)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		logger.Info("hook received", "payload", payload)

		// If event is command, mark the agent as thinking
		if (payload.Event == "command" || payload.Event == "tool_use") && payload.SessionKey != "" {
			parts := strings.Split(payload.SessionKey, ":")
			if len(parts) > 0 {
				agentID := parts[0]
				if dp != nil {
					dp.MarkAgentActive(agentID, "thinking")
				}
			}
		}

		// Broadcast a refresh event to all frontend clients
		b.Broadcast(data.Event{
			Type:    "refresh",
			Payload: payload,
		})

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}
}

func handleSessionList(logger *slog.Logger, dp *data.DataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agentId")
		if agentID == "" {
			http.Error(w, "Missing agentId", http.StatusBadRequest)
			return
		}

		sessions, err := dp.GetSessionsForAgent(agentID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	}
}

func handleSessionDetails(logger *slog.Logger, dp *data.DataProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		agentID := r.URL.Query().Get("agentId")
		sessionID := r.URL.Query().Get("sessionId")
		
		if agentID == "" || sessionID == "" {
			http.Error(w, "Missing agentId or sessionId", http.StatusBadRequest)
			return
		}

		details, err := dp.GetSessionDetails(agentID, sessionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(details)
	}
}
