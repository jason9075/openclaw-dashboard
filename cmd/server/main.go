package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"openclaw/internal/data"
	"openclaw/internal/monitor"
	"openclaw/internal/ui"
)

type DashboardState struct {
	Timestamp   time.Time               `json:"timestamp"`
	Uptime      string                  `json:"uptime"`
	System      monitor.SystemStats     `json:"system"`
	Todos       []data.TodoItem         `json:"todos"`
	Sessions    map[string]data.Session `json:"sessions"`
	Alerts      []data.Alert            `json:"alerts"`
	GitLog      []data.GitCommit        `json:"git_log"`
	CronJobs    []data.CronJob          `json:"cron_jobs"`
	Costs       []data.CostCard         `json:"costs"`
	TokenUsage  []data.TokenStats       `json:"token_usage"`
	SubAgents   []data.SubAgentRun      `json:"sub_agents"`
	Models      []data.Model            `json:"models"`
	Skills      []data.Skill            `json:"skills"`
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
		// We don't exit, just continue with empty data potentially
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
		var subAgents []data.SubAgentRun
		var models []data.Model
		var skills []data.Skill

		if dp != nil {
			todos, _ = dp.GetTodos()
			sessions, _ = dp.GetActiveSessions()
			alerts, _ = dp.GetAlerts()
			gitLog, _ = dp.GetGitLog()
			cronJobs, _ = dp.GetCronJobs()
			costs, _ = dp.GetCosts()
			tokenUsage, _ = dp.GetTokenUsage()
			subAgents, _ = dp.GetSubAgentActivity()
			models, _ = dp.GetModels()
			skills, _ = dp.GetSkills()
		}

		state := DashboardState{
			Timestamp: time.Now(),
			Uptime:    time.Since(startTime).String(),
			System:    sysStats,
			Todos:     todos,
			Sessions:  sessions,
			Alerts:    alerts,
			GitLog:    gitLog,
			CronJobs:  cronJobs,
			Costs:     costs,
			TokenUsage: tokenUsage,
			SubAgents: subAgents,
			Models:    models,
			Skills:    skills,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(state); err != nil {
			logger.Error("failed to encode state", "error", err)
		}
	}
}

func handleAgents(uiFS fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Serve agents.html
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
