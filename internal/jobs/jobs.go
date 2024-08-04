// Package jobs provides a background job processing system.
package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrJobNotFound      = errors.New("job not found")
	ErrJobAlreadyExists = errors.New("job already exists")
	ErrQueueClosed      = errors.New("job queue is closed")
	ErrMaxAttemptsExceeded = errors.New("max attempts exceeded")
)

// JobStatus defines job states.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusRetrying  JobStatus = "retrying"
)

// Job represents a background job.
type Job struct {
	ID          int64           `json:"id"`
	Queue       string          `json:"queue"`
	JobType     string          `json:"job_type"`
	Payload     json.RawMessage `json:"payload"`
	Status      JobStatus       `json:"status"`
	Priority    int             `json:"priority"`
	Attempts    int             `json:"attempts"`
	MaxAttempts int             `json:"max_attempts"`
	LastError   string          `json:"last_error,omitempty"`
	ScheduledAt int64           `json:"scheduled_at"`
	StartedAt   *int64          `json:"started_at,omitempty"`
	CompletedAt *int64          `json:"completed_at,omitempty"`
	CreatedAt   int64           `json:"created_at"`
}

// Handler is a function that processes a job.
type Handler func(ctx context.Context, job *Job) error

// Config holds job queue configuration.
type Config struct {
	WorkerCount    int           `json:"worker_count" yaml:"worker_count"`
	PollInterval   time.Duration `json:"poll_interval" yaml:"poll_interval"`
	MaxRetries     int           `json:"max_retries" yaml:"max_retries"`
	RetryDelay     time.Duration `json:"retry_delay" yaml:"retry_delay"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout" yaml:"shutdown_timeout"`
}

// DefaultConfig returns default configuration.
func DefaultConfig() Config {
	return Config{
		WorkerCount:     4,
		PollInterval:    5 * time.Second,
		MaxRetries:      3,
		RetryDelay:      30 * time.Second,
		ShutdownTimeout: 30 * time.Second,
	}
}

// Queue manages background job processing.
type Queue struct {
	db       *sql.DB
	config   Config
	handlers map[string]Handler
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	running  bool
}

// NewQueue creates a new job queue.
func NewQueue(db *sql.DB, config Config) *Queue {
	if config.WorkerCount <= 0 {
		config.WorkerCount = 4
	}
	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay <= 0 {
		config.RetryDelay = 30 * time.Second
	}
	if config.ShutdownTimeout <= 0 {
		config.ShutdownTimeout = 30 * time.Second
	}

	return &Queue{
		db:       db,
		config:   config,
		handlers: make(map[string]Handler),
	}
}

// RegisterHandler registers a handler for a job type.
func (q *Queue) RegisterHandler(jobType string, handler Handler) {
	q.mu.Lock()
	q.handlers[jobType] = handler
	q.mu.Unlock()
}

// Start starts the job queue workers.
func (q *Queue) Start() {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return
	}
	q.running = true
	q.ctx, q.cancel = context.WithCancel(context.Background())
	q.mu.Unlock()

	// Start workers
	for i := 0; i < q.config.WorkerCount; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

// Stop stops the job queue gracefully.
func (q *Queue) Stop() {
	q.mu.Lock()
	if !q.running {
		q.mu.Unlock()
		return
	}
	q.running = false
	q.cancel()
	q.mu.Unlock()

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		q.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(q.config.ShutdownTimeout):
	}
}

// worker processes jobs from the queue.
func (q *Queue) worker(id int) {
	defer q.wg.Done()

	for {
		select {
		case <-q.ctx.Done():
			return
		default:
		}

		job, err := q.fetchJob()
		if err != nil || job == nil {
			select {
			case <-q.ctx.Done():
				return
			case <-time.After(q.config.PollInterval):
				continue
			}
		}

		q.processJob(job)
	}
}

// fetchJob fetches the next pending job.
func (q *Queue) fetchJob() (*Job, error) {
	now := time.Now().Unix()

	// Try to claim a job atomically
	result, err := q.db.Exec(`
		UPDATE jobs SET status = ?, started_at = ?
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = ? AND scheduled_at <= ?
			ORDER BY priority DESC, scheduled_at ASC
			LIMIT 1
		)`,
		JobStatusRunning, now, JobStatusPending, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to claim job: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, nil
	}

	// Fetch the claimed job
	var job Job
	var lastError sql.NullString
	var startedAt, completedAt sql.NullInt64

	err = q.db.QueryRow(`
		SELECT id, queue, job_type, payload, status, priority, attempts, max_attempts, last_error, scheduled_at, started_at, completed_at, created_at
		FROM jobs WHERE status = ? ORDER BY started_at DESC LIMIT 1`,
		JobStatusRunning,
	).Scan(&job.ID, &job.Queue, &job.JobType, &job.Payload, &job.Status, &job.Priority, &job.Attempts, &job.MaxAttempts, &lastError, &job.ScheduledAt, &startedAt, &completedAt, &job.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch job: %w", err)
	}

	if lastError.Valid {
		job.LastError = lastError.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Int64
	}

	return &job, nil
}

// processJob processes a single job.
func (q *Queue) processJob(job *Job) {
	q.mu.RLock()
	handler, exists := q.handlers[job.JobType]
	q.mu.RUnlock()

	if !exists {
		q.failJob(job, fmt.Errorf("no handler registered for job type: %s", job.JobType))
		return
	}

	// Update attempts
	job.Attempts++
	q.db.Exec("UPDATE jobs SET attempts = ? WHERE id = ?", job.Attempts, job.ID)

	// Execute handler with timeout
	ctx, cancel := context.WithTimeout(q.ctx, 5*time.Minute)
	defer cancel()

	err := handler(ctx, job)
	if err != nil {
		q.failJob(job, err)
	} else {
		q.completeJob(job)
	}
}

// completeJob marks a job as completed.
func (q *Queue) completeJob(job *Job) {
	now := time.Now().Unix()
	q.db.Exec(
		"UPDATE jobs SET status = ?, completed_at = ? WHERE id = ?",
		JobStatusCompleted, now, job.ID,
	)
}

// failJob marks a job as failed or schedules a retry.
func (q *Queue) failJob(job *Job, err error) {
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	if job.Attempts < job.MaxAttempts {
		// Schedule retry
		retryAt := time.Now().Add(q.config.RetryDelay * time.Duration(job.Attempts)).Unix()
		q.db.Exec(
			"UPDATE jobs SET status = ?, last_error = ?, scheduled_at = ? WHERE id = ?",
			JobStatusPending, errStr, retryAt, job.ID,
		)
	} else {
		// Max attempts exceeded
		now := time.Now().Unix()
		q.db.Exec(
			"UPDATE jobs SET status = ?, last_error = ?, completed_at = ? WHERE id = ?",
			JobStatusFailed, errStr, now, job.ID,
		)
	}
}

// Enqueue adds a job to the queue.
func (q *Queue) Enqueue(jobType string, payload any, opts ...EnqueueOption) (int64, error) {
	options := enqueueOptions{
		queue:       "default",
		priority:    0,
		maxAttempts: q.config.MaxRetries,
		scheduledAt: time.Now(),
	}

	for _, opt := range opts {
		opt(&options)
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal payload: %w", err)
	}

	now := time.Now().Unix()
	result, err := q.db.Exec(`
		INSERT INTO jobs (queue, job_type, payload, status, priority, max_attempts, scheduled_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		options.queue, jobType, string(payloadJSON), JobStatusPending, options.priority, options.maxAttempts, options.scheduledAt.Unix(), now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return result.LastInsertId()
}

// EnqueueOption is a function that configures job enqueue options.
type EnqueueOption func(*enqueueOptions)

type enqueueOptions struct {
	queue       string
	priority    int
	maxAttempts int
	scheduledAt time.Time
}

// WithQueue sets the job queue.
func WithQueue(queue string) EnqueueOption {
	return func(o *enqueueOptions) {
		o.queue = queue
	}
}

// WithPriority sets the job priority.
func WithPriority(priority int) EnqueueOption {
	return func(o *enqueueOptions) {
		o.priority = priority
	}
}

// WithMaxAttempts sets the maximum retry attempts.
func WithMaxAttempts(attempts int) EnqueueOption {
	return func(o *enqueueOptions) {
		o.maxAttempts = attempts
	}
}

// WithSchedule schedules the job for a specific time.
func WithSchedule(t time.Time) EnqueueOption {
	return func(o *enqueueOptions) {
		o.scheduledAt = t
	}
}

// WithDelay delays the job by a duration.
func WithDelay(d time.Duration) EnqueueOption {
	return func(o *enqueueOptions) {
		o.scheduledAt = time.Now().Add(d)
	}
}

// GetJob retrieves a job by ID.
func (q *Queue) GetJob(id int64) (*Job, error) {
	var job Job
	var lastError sql.NullString
	var startedAt, completedAt sql.NullInt64

	err := q.db.QueryRow(`
		SELECT id, queue, job_type, payload, status, priority, attempts, max_attempts, last_error, scheduled_at, started_at, completed_at, created_at
		FROM jobs WHERE id = ?`,
		id,
	).Scan(&job.ID, &job.Queue, &job.JobType, &job.Payload, &job.Status, &job.Priority, &job.Attempts, &job.MaxAttempts, &lastError, &job.ScheduledAt, &startedAt, &completedAt, &job.CreatedAt)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if lastError.Valid {
		job.LastError = lastError.String
	}
	if startedAt.Valid {
		job.StartedAt = &startedAt.Int64
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Int64
	}

	return &job, nil
}

// ListJobs lists jobs with optional filters.
func (q *Queue) ListJobs(queue string, status *JobStatus, limit int) ([]Job, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := "SELECT id, queue, job_type, payload, status, priority, attempts, max_attempts, last_error, scheduled_at, started_at, completed_at, created_at FROM jobs WHERE 1=1"
	args := []any{}

	if queue != "" {
		query += " AND queue = ?"
		args = append(args, queue)
	}
	if status != nil {
		query += " AND status = ?"
		args = append(args, *status)
	}
	query += " ORDER BY priority DESC, scheduled_at ASC LIMIT ?"
	args = append(args, limit)

	rows, err := q.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		var job Job
		var lastError sql.NullString
		var startedAt, completedAt sql.NullInt64

		err := rows.Scan(&job.ID, &job.Queue, &job.JobType, &job.Payload, &job.Status, &job.Priority, &job.Attempts, &job.MaxAttempts, &lastError, &job.ScheduledAt, &startedAt, &completedAt, &job.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		if lastError.Valid {
			job.LastError = lastError.String
		}
		if startedAt.Valid {
			job.StartedAt = &startedAt.Int64
		}
		if completedAt.Valid {
			job.CompletedAt = &completedAt.Int64
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Stats returns queue statistics.
type Stats struct {
	TotalJobs     int `json:"total_jobs"`
	PendingJobs   int `json:"pending_jobs"`
	RunningJobs   int `json:"running_jobs"`
	CompletedJobs int `json:"completed_jobs"`
	FailedJobs    int `json:"failed_jobs"`
}

// GetStats returns queue statistics.
func (q *Queue) GetStats() (*Stats, error) {
	var s Stats

	q.db.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&s.TotalJobs)
	q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", JobStatusPending).Scan(&s.PendingJobs)
	q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", JobStatusRunning).Scan(&s.RunningJobs)
	q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", JobStatusCompleted).Scan(&s.CompletedJobs)
	q.db.QueryRow("SELECT COUNT(*) FROM jobs WHERE status = ?", JobStatusFailed).Scan(&s.FailedJobs)

	return &s, nil
}

// Cleanup removes old completed/failed jobs.
func (q *Queue) Cleanup(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays).Unix()
	result, err := q.db.Exec(
		"DELETE FROM jobs WHERE (status = ? OR status = ?) AND completed_at < ?",
		JobStatusCompleted, JobStatusFailed, cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup jobs: %w", err)
	}
	return result.RowsAffected()
}

// RetryJob retries a failed job.
func (q *Queue) RetryJob(id int64) error {
	job, err := q.GetJob(id)
	if err != nil {
		return err
	}

	if job.Status != JobStatusFailed {
		return errors.New("can only retry failed jobs")
	}

	now := time.Now().Unix()
	_, err = q.db.Exec(
		"UPDATE jobs SET status = ?, attempts = 0, scheduled_at = ?, started_at = NULL, completed_at = NULL WHERE id = ?",
		JobStatusPending, now, id,
	)
	return err
}
