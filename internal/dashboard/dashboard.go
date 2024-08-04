// Package dashboard provides a web-based administration interface for tonkey.
package dashboard

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strconv"
	"time"
)

//go:embed templates/layout.html
var layoutHTML string

//go:embed templates/dashboard.html
var dashboardHTML string

//go:embed templates/users.html
var usersHTML string

//go:embed templates/transactions.html
var transactionsHTML string

//go:embed templates/webhooks.html
var webhooksHTML string

//go:embed templates/audit.html
var auditHTML string

//go:embed templates/jobs.html
var jobsHTML string

//go:embed templates/settings.html
var settingsHTML string

//go:embed static/dashboard.css
var dashboardCSS string

//go:embed static/dashboard.js
var dashboardJS string

// Dashboard represents the admin dashboard server.
type Dashboard struct {
	db        *sql.DB
	templates *template.Template
}

// DashboardStats holds overview statistics.
type DashboardStats struct {
	TotalUsers        int64   `json:"total_users"`
	ActiveUsers       int64   `json:"active_users"`
	TotalTransactions int64   `json:"total_transactions"`
	TotalVolume       int64   `json:"total_volume"`
	ActiveWebhooks    int64   `json:"active_webhooks"`
	PendingJobs       int64   `json:"pending_jobs"`
	FailedJobs        int64   `json:"failed_jobs"`
	AuditLogsToday    int64   `json:"audit_logs_today"`
	Uptime            string  `json:"uptime"`
	Version           string  `json:"version"`
}

// New creates a new Dashboard instance.
func New(db *sql.DB) *Dashboard {
	funcMap := template.FuncMap{
		"formatTime": func(ts int64) string {
			return time.Unix(ts, 0).Format("2006-01-02 15:04:05")
		},
		"formatDuration": func(seconds int64) string {
			d := time.Duration(seconds) * time.Second
			days := int(d.Hours() / 24)
			hours := int(d.Hours()) % 24
			minutes := int(d.Minutes()) % 60
			if days > 0 {
				return strconv.Itoa(days) + "d " + strconv.Itoa(hours) + "h"
			}
			return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
		},
		"formatAmount": func(nanoTON int64) string {
			ton := float64(nanoTON) / 1e9
			return strconv.FormatFloat(ton, 'f', 2, 64) + " TON"
		},
		"json": func(v any) string {
			b, _ := json.Marshal(v)
			return string(b)
		},
	}

	tmpl := template.New("dashboard").Funcs(funcMap)
	template.Must(tmpl.New("layout").Parse(layoutHTML))
	template.Must(tmpl.New("dashboard").Parse(dashboardHTML))
	template.Must(tmpl.New("users").Parse(usersHTML))
	template.Must(tmpl.New("transactions").Parse(transactionsHTML))
	template.Must(tmpl.New("webhooks").Parse(webhooksHTML))
	template.Must(tmpl.New("audit").Parse(auditHTML))
	template.Must(tmpl.New("jobs").Parse(jobsHTML))
	template.Must(tmpl.New("settings").Parse(settingsHTML))

	return &Dashboard{
		db:        db,
		templates: tmpl,
	}
}

// Register registers the dashboard routes.
func (d *Dashboard) Register(mux *http.ServeMux) {
	// Static assets
	mux.HandleFunc("/admin/dashboard/static/dashboard.css", d.handleCSS)
	mux.HandleFunc("/admin/dashboard/static/dashboard.js", d.handleJS)

	// Dashboard pages
	mux.HandleFunc("/admin/dashboard", d.handleDashboard)
	mux.HandleFunc("/admin/dashboard/", d.handleDashboard)
	mux.HandleFunc("/admin/dashboard/users", d.handleUsers)
	mux.HandleFunc("/admin/dashboard/transactions", d.handleTransactions)
	mux.HandleFunc("/admin/dashboard/webhooks", d.handleWebhooks)
	mux.HandleFunc("/admin/dashboard/audit", d.handleAudit)
	mux.HandleFunc("/admin/dashboard/jobs", d.handleJobs)
	mux.HandleFunc("/admin/dashboard/settings", d.handleSettings)

	// API endpoints for HTMX
	mux.HandleFunc("/admin/dashboard/api/stats", d.handleAPIStats)
	mux.HandleFunc("/admin/dashboard/api/users", d.handleAPIUsers)
	mux.HandleFunc("/admin/dashboard/api/transactions", d.handleAPITransactions)
	mux.HandleFunc("/admin/dashboard/api/activity", d.handleAPIActivity)
}

func (d *Dashboard) handleCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css")
	w.Write([]byte(dashboardCSS))
}

func (d *Dashboard) handleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript")
	w.Write([]byte(dashboardJS))
}

func (d *Dashboard) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats := d.getStats()
	d.render(w, "dashboard", map[string]any{
		"Title":    "Dashboard",
		"Active":   "dashboard",
		"Stats":    stats,
		"Activity": d.getRecentActivity(10),
	})
}

func (d *Dashboard) handleUsers(w http.ResponseWriter, r *http.Request) {
	d.render(w, "users", map[string]any{
		"Title":  "Users",
		"Active": "users",
		"Users":  d.getUsers(50, 0),
	})
}

func (d *Dashboard) handleTransactions(w http.ResponseWriter, r *http.Request) {
	d.render(w, "transactions", map[string]any{
		"Title":        "Transactions",
		"Active":       "transactions",
		"Transactions": d.getTransactions(50, 0),
	})
}

func (d *Dashboard) handleWebhooks(w http.ResponseWriter, r *http.Request) {
	d.render(w, "webhooks", map[string]any{
		"Title":    "Webhooks",
		"Active":   "webhooks",
		"Webhooks": d.getWebhooks(50, 0),
	})
}

func (d *Dashboard) handleAudit(w http.ResponseWriter, r *http.Request) {
	d.render(w, "audit", map[string]any{
		"Title":  "Audit Logs",
		"Active": "audit",
		"Logs":   d.getAuditLogs(100, 0),
	})
}

func (d *Dashboard) handleJobs(w http.ResponseWriter, r *http.Request) {
	d.render(w, "jobs", map[string]any{
		"Title":  "Background Jobs",
		"Active": "jobs",
		"Jobs":   d.getJobs(50, 0),
	})
}

func (d *Dashboard) handleSettings(w http.ResponseWriter, r *http.Request) {
	d.render(w, "settings", map[string]any{
		"Title":  "Settings",
		"Active": "settings",
	})
}

func (d *Dashboard) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	stats := d.getStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (d *Dashboard) handleAPIUsers(w http.ResponseWriter, r *http.Request) {
	users := d.getUsers(50, 0)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (d *Dashboard) handleAPITransactions(w http.ResponseWriter, r *http.Request) {
	txs := d.getTransactions(50, 0)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(txs)
}

func (d *Dashboard) handleAPIActivity(w http.ResponseWriter, r *http.Request) {
	activity := d.getRecentActivity(10)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activity)
}

func (d *Dashboard) render(w http.ResponseWriter, name string, data map[string]any) {
	data["Version"] = "1.1.0"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.templates.ExecuteTemplate(w, "layout", map[string]any{
		"Content": name,
		"Data":    data,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d *Dashboard) getStats() *DashboardStats {
	stats := &DashboardStats{
		Version: "1.1.0",
		Uptime:  "N/A",
	}

	// Total users
	d.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.TotalUsers)

	// Active users (logged in last 7 days)
	weekAgo := time.Now().AddDate(0, 0, -7).Unix()
	d.db.QueryRow("SELECT COUNT(*) FROM users WHERE last_login > ?", weekAgo).Scan(&stats.ActiveUsers)

	// Total transactions
	d.db.QueryRow("SELECT COUNT(*) FROM tx_log").Scan(&stats.TotalTransactions)

	// Total volume
	d.db.QueryRow("SELECT COALESCE(SUM(amount), 0) FROM tx_log").Scan(&stats.TotalVolume)

	// Active webhooks
	d.db.QueryRow("SELECT COUNT(*) FROM webhooks WHERE is_active = 1").Scan(&stats.ActiveWebhooks)

	// Pending and failed jobs
	d.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = 'pending'").Scan(&stats.PendingJobs)
	d.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = 'failed'").Scan(&stats.FailedJobs)

	// Audit logs today
	todayStart := time.Now().Truncate(24 * time.Hour).Unix()
	d.db.QueryRow("SELECT COUNT(*) FROM audit_logs WHERE created_at > ?", todayStart).Scan(&stats.AuditLogsToday)

	return stats
}

type User struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	IsActive  bool   `json:"is_active"`
	CreatedAt int64  `json:"created_at"`
	LastLogin *int64 `json:"last_login"`
}

func (d *Dashboard) getUsers(limit, offset int) []User {
	rows, err := d.db.Query(`
		SELECT id, username, COALESCE(email, ''), role, is_active, created_at, last_login
		FROM users
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.IsActive, &u.CreatedAt, &u.LastLogin)
		users = append(users, u)
	}
	return users
}

type Transaction struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Amount    int64  `json:"amount"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

func (d *Dashboard) getTransactions(limit, offset int) []Transaction {
	rows, err := d.db.Query(`
		SELECT id, "from", "to", amount, status, created_at
		FROM tx_log
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		var tx Transaction
		rows.Scan(&tx.ID, &tx.From, &tx.To, &tx.Amount, &tx.Status, &tx.CreatedAt)
		txs = append(txs, tx)
	}
	return txs
}

type Webhook struct {
	ID            int64  `json:"id"`
	URL           string `json:"url"`
	EventType     string `json:"event_type"`
	FilterAddress string `json:"filter_address"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     int64  `json:"created_at"`
}

func (d *Dashboard) getWebhooks(limit, offset int) []Webhook {
	rows, err := d.db.Query(`
		SELECT id, url, event_type, COALESCE(filter_address, ''), is_active, created_at
		FROM webhooks
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var webhooks []Webhook
	for rows.Next() {
		var wh Webhook
		rows.Scan(&wh.ID, &wh.URL, &wh.EventType, &wh.FilterAddress, &wh.IsActive, &wh.CreatedAt)
		webhooks = append(webhooks, wh)
	}
	return webhooks
}

type AuditLog struct {
	ID           int64   `json:"id"`
	UserID       *int64  `json:"user_id"`
	Action       string  `json:"action"`
	ResourceType *string `json:"resource_type"`
	ResourceID   *string `json:"resource_id"`
	IPAddress    *string `json:"ip_address"`
	CreatedAt    int64   `json:"created_at"`
}

func (d *Dashboard) getAuditLogs(limit, offset int) []AuditLog {
	rows, err := d.db.Query(`
		SELECT id, user_id, action, resource_type, resource_id, ip_address, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		rows.Scan(&l.ID, &l.UserID, &l.Action, &l.ResourceType, &l.ResourceID, &l.IPAddress, &l.CreatedAt)
		logs = append(logs, l)
	}
	return logs
}

type Job struct {
	ID          int64   `json:"id"`
	JobType     string  `json:"job_type"`
	Queue       string  `json:"queue"`
	Status      string  `json:"status"`
	Priority    int     `json:"priority"`
	Attempts    int     `json:"attempts"`
	MaxAttempts int     `json:"max_attempts"`
	LastError   *string `json:"last_error"`
	CreatedAt   int64   `json:"created_at"`
}

func (d *Dashboard) getJobs(limit, offset int) []Job {
	rows, err := d.db.Query(`
		SELECT id, job_type, queue, status, priority, attempts, max_attempts, last_error, created_at
		FROM jobs
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, limit, offset)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var j Job
		rows.Scan(&j.ID, &j.JobType, &j.Queue, &j.Status, &j.Priority, &j.Attempts, &j.MaxAttempts, &j.LastError, &j.CreatedAt)
		jobs = append(jobs, j)
	}
	return jobs
}

type Activity struct {
	Type      string `json:"type"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func (d *Dashboard) getRecentActivity(limit int) []Activity {
	var activities []Activity

	// Get recent audit logs as activity
	rows, err := d.db.Query(`
		SELECT action, resource_type, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return activities
	}
	defer rows.Close()

	for rows.Next() {
		var action, resourceType string
		var createdAt int64
		rows.Scan(&action, &resourceType, &createdAt)
		activities = append(activities, Activity{
			Type:      action,
			Message:   action + " on " + resourceType,
			Timestamp: createdAt,
		})
	}

	return activities
}
