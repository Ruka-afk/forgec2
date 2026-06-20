package logger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// StructuredLogger provides enhanced logging with context
type StructuredLogger struct {
	logger *slog.Logger
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(logDir string, level slog.Level) (*StructuredLogger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	// Create log file with date rotation
	logFile := filepath.Join(logDir, fmt.Sprintf("forgec2_%s.log", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	// Create multi-writer (console + file)
	handler := slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: level,
	})

	logger := slog.New(handler)

	return &StructuredLogger{logger: logger}, nil
}

// Info logs an info message
func (sl *StructuredLogger) Info(msg string, args ...any) {
	sl.logger.Info(msg, args...)
}

// Error logs an error message
func (sl *StructuredLogger) Error(msg string, args ...any) {
	sl.logger.Error(msg, args...)
}

// Warn logs a warning message
func (sl *StructuredLogger) Warn(msg string, args ...any) {
	sl.logger.Warn(msg, args...)
}

// Debug logs a debug message
func (sl *StructuredLogger) Debug(msg string, args ...any) {
	sl.logger.Debug(msg, args...)
}

// WithContext adds context to the logger
func (sl *StructuredLogger) WithContext(ctx context.Context) *StructuredLogger {
	// Extract request ID, user ID, etc. from context
	return sl
}

// WithAgent adds agent context
func (sl *StructuredLogger) WithAgent(agentID string) *StructuredLogger {
	return &StructuredLogger{
		logger: sl.logger.With("agent_id", agentID),
	}
}

// WithUser adds user context
func (sl *StructuredLogger) WithUser(userID string) *StructuredLogger {
	return &StructuredLogger{
		logger: sl.logger.With("user_id", userID),
	}
}

// WithTask adds task context
func (sl *StructuredLogger) WithTask(taskID uint) *StructuredLogger {
	return &StructuredLogger{
		logger: sl.logger.With("task_id", taskID),
	}
}

// AppError represents a structured application error
type AppError struct {
	Code       string
	Message    string
	Cause      error
	StatusCode int
	Context    map[string]interface{}
	Timestamp  time.Time
	Stack      string
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new application error
func NewAppError(code, message string, cause error) *AppError {
	// Capture stack trace
	_, file, line, _ := runtime.Caller(1)

	return &AppError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
		Stack:     fmt.Sprintf("%s:%d", file, line),
		Context:   make(map[string]interface{}),
	}
}

// WithContext adds context to the error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	e.Context[key] = value
	return e
}

// WithStatusCode sets the HTTP status code
func (e *AppError) WithStatusCode(code int) *AppError {
	e.StatusCode = code
	return e
}

// Common error types
var (
	ErrNotFound         = errors.New("resource not found")
	ErrUnauthorized     = errors.New("unauthorized access")
	ErrForbidden        = errors.New("access forbidden")
	ErrBadRequest       = errors.New("invalid request")
	ErrInternal         = errors.New("internal server error")
	ErrAgentOffline     = errors.New("agent is offline")
	ErrTaskFailed       = errors.New("task execution failed")
	ErrDatabaseError    = errors.New("database operation failed")
	ErrFileNotFound     = errors.New("file not found")
	ErrPermissionDenied = errors.New("permission denied")
)

// ErrorTracker tracks and aggregates errors
type ErrorTracker struct {
	errors map[string]*ErrorAggregate
}

// ErrorAggregate contains aggregated error information
type ErrorAggregate struct {
	Code      string
	Message   string
	Count     int64
	FirstSeen time.Time
	LastSeen  time.Time
	Stack     string
	Contexts  []map[string]interface{}
}

// NewErrorTracker creates a new error tracker
func NewErrorTracker() *ErrorTracker {
	return &ErrorTracker{
		errors: make(map[string]*ErrorAggregate),
	}
}

// Track records an error
func (et *ErrorTracker) Track(err *AppError) {
	if existing, exists := et.errors[err.Code]; exists {
		existing.Count++
		existing.LastSeen = time.Now()
		if len(et.errors[err.Code].Contexts) < 10 {
			existing.Contexts = append(existing.Contexts, err.Context)
		}
	} else {
		et.errors[err.Code] = &ErrorAggregate{
			Code:      err.Code,
			Message:   err.Message,
			Count:     1,
			FirstSeen: err.Timestamp,
			LastSeen:  err.Timestamp,
			Stack:     err.Stack,
			Contexts:  []map[string]interface{}{err.Context},
		}
	}
}

// GetErrors returns all tracked errors
func (et *ErrorTracker) GetErrors() map[string]*ErrorAggregate {
	return et.errors
}

// GetTopErrors returns the most frequent errors
func (et *ErrorTracker) GetTopErrors(limit int) []*ErrorAggregate {
	var top []*ErrorAggregate

	for _, agg := range et.errors {
		top = append(top, agg)
	}

	// Sort by count (simple bubble sort)
	for i := 0; i < len(top)-1; i++ {
		for j := i + 1; j < len(top); j++ {
			if top[i].Count < top[j].Count {
				top[i], top[j] = top[j], top[i]
			}
		}
	}

	if limit > len(top) {
		limit = len(top)
	}

	return top[:limit]
}

// Reset clears all tracked errors
func (et *ErrorTracker) Reset() {
	et.errors = make(map[string]*ErrorAggregate)
}

// Middleware provides error handling middleware
type Middleware struct {
	logger  *StructuredLogger
	tracker *ErrorTracker
}

// NewMiddleware creates a new error middleware
func NewMiddleware(logger *StructuredLogger, tracker *ErrorTracker) *Middleware {
	return &Middleware{
		logger:  logger,
		tracker: tracker,
	}
}

// HandleError processes an error and logs it appropriately
func (m *Middleware) HandleError(err error, context map[string]interface{}) {
	if appErr, ok := err.(*AppError); ok {
		// Structured error
		m.logger.Error(appErr.Message,
			"code", appErr.Code,
			"status", appErr.StatusCode,
			"context", appErr.Context,
			"stack", appErr.Stack,
		)
		m.tracker.Track(appErr)
	} else {
		// Generic error
		m.logger.Error("Unexpected error",
			"error", err.Error(),
			"context", context,
		)

		// Convert to AppError for tracking
		appErr := NewAppError("UNKNOWN", err.Error(), err)
		appErr.Context = context
		m.tracker.Track(appErr)
	}
}

// RequestLogger logs HTTP requests
type RequestLogger struct {
	logger *StructuredLogger
}

// NewRequestLogger creates a new request logger
func NewRequestLogger(logger *StructuredLogger) *RequestLogger {
	return &RequestLogger{logger: logger}
}

// LogRequest logs an HTTP request
func (rl *RequestLogger) LogRequest(method, path string, statusCode int, duration time.Duration, context map[string]interface{}) {
	rl.logger.Info("HTTP request",
		"method", method,
		"path", path,
		"status", statusCode,
		"duration_ms", duration.Milliseconds(),
		"context", context,
	)
}

// LogError logs an HTTP error
func (rl *RequestLogger) LogError(method, path string, err error, context map[string]interface{}) {
	rl.logger.Error("HTTP error",
		"method", method,
		"path", path,
		"error", err.Error(),
		"context", context,
	)
}

// PerformanceLogger logs performance metrics
type PerformanceLogger struct {
	logger *StructuredLogger
}

// NewPerformanceLogger creates a new performance logger
func NewPerformanceLogger(logger *StructuredLogger) *PerformanceLogger {
	return &PerformanceLogger{logger: logger}
}

// LogQuery logs a database query
func (pl *PerformanceLogger) LogQuery(query string, duration time.Duration, rows int64) {
	if duration > 100*time.Millisecond {
		pl.logger.Warn("Slow database query",
			"query", query,
			"duration_ms", duration.Milliseconds(),
			"rows", rows,
		)
	} else {
		pl.logger.Debug("Database query",
			"query", query,
			"duration_ms", duration.Milliseconds(),
			"rows", rows,
		)
	}
}

// LogTask logs a task execution
func (pl *PerformanceLogger) LogTask(taskType string, duration time.Duration, success bool) {
	pl.logger.Info("Task executed",
		"type", taskType,
		"duration_ms", duration.Milliseconds(),
		"success", success,
	)
}

// LogBeacon logs a beacon check-in
func (pl *PerformanceLogger) LogBeacon(agentID string, duration time.Duration) {
	pl.logger.Debug("Beacon processed",
		"agent_id", agentID,
		"duration_ms", duration.Milliseconds(),
	)
}
