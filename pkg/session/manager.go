package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	gocache "github.com/patrickmn/go-cache"
	"go.uber.org/zap"
)

// Context represents a session context
type Context struct {
	ID           string            `json:"id"`
	Headers      map[string]string `json:"headers"`
	CreatedAt    time.Time         `json:"created_at"`
	LastAccessed time.Time         `json:"last_accessed"`
	CallCount    int64             `json:"call_count"`
	UserAgent    string            `json:"user_agent"`
	RemoteAddr   string            `json:"remote_addr"`

	// Rate limiting
	RequestCount int64     `json:"request_count"`
	WindowStart  time.Time `json:"window_start"`

	// Security
	IsBlocked bool `json:"is_blocked"`

	// Synchronization
	mu sync.RWMutex
}

// Manager manages user sessions
type Manager struct {
	cache  *gocache.Cache
	logger *zap.Logger
	mu     sync.RWMutex

	// Configuration
	defaultExpiration time.Duration
	cleanupInterval   time.Duration
	maxSessions       int

	// Rate limiting
	requestsPerMinute int
	windowSize        time.Duration
}

// NewManager creates a new session manager
func NewManager(logger *zap.Logger) *Manager {
	defaultExpiration := 30 * time.Minute
	cleanupInterval := 5 * time.Minute

	return &Manager{
		cache:             gocache.New(defaultExpiration, cleanupInterval),
		logger:            logger,
		defaultExpiration: defaultExpiration,
		cleanupInterval:   cleanupInterval,
		maxSessions:       10000,
		requestsPerMinute: 100,
		windowSize:        time.Minute,
	}
}

// GetOrCreateSession gets an existing session or creates a new one
func (m *Manager) GetOrCreateSession(sessionID string, headers map[string]string) *Context {
	// If no session ID provided, create a new session
	if sessionID == "" {
		return m.CreateSession(headers)
	}

	// Try to get existing session
	if ctx, exists := m.GetSession(sessionID); exists {
		// Update last accessed time
		ctx.UpdateLastAccessed()
		return ctx
	}

	// Session not found, create new one
	return m.CreateSession(headers)
}

// CreateSession creates a new session
func (m *Manager) CreateSession(headers map[string]string) *Context {
	// Check if we're at the session limit
	if m.cache.ItemCount() >= m.maxSessions {
		m.logger.Warn("Session limit reached", zap.Int("current", m.cache.ItemCount()), zap.Int("max", m.maxSessions))
		m.cleanup()
	}

	sessionID := m.generateSessionID()

	ctx := &Context{
		ID:           sessionID,
		Headers:      headers,
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		CallCount:    0,
		UserAgent:    headers["User-Agent"],
		RemoteAddr:   headers["X-Real-IP"],
		RequestCount: 0,
		WindowStart:  time.Now(),
		IsBlocked:    false,
	}

	// If no remote address in headers, try other headers
	if ctx.RemoteAddr == "" {
		ctx.RemoteAddr = headers["X-Forwarded-For"]
	}

	m.cache.Set(sessionID, ctx, m.defaultExpiration)

	m.logger.Info("Created new session",
		zap.String("sessionId", sessionID),
		zap.String("userAgent", ctx.UserAgent),
		zap.String("remoteAddr", ctx.RemoteAddr))

	return ctx
}

// GetSession retrieves a session by ID
func (m *Manager) GetSession(sessionID string) (*Context, bool) {
	if item, exists := m.cache.Get(sessionID); exists {
		if ctx, ok := item.(*Context); ok {
			return ctx, true
		}
	}
	return nil, false
}

// UpdateSession updates an existing session
func (m *Manager) UpdateSession(sessionID string, ctx *Context) {
	m.cache.Set(sessionID, ctx, m.defaultExpiration)
}

// DeleteSession removes a session
func (m *Manager) DeleteSession(sessionID string) {
	m.cache.Delete(sessionID)
	m.logger.Info("Deleted session", zap.String("sessionId", sessionID))
}

// BlockSession blocks a session
func (m *Manager) BlockSession(sessionID string) {
	if ctx, exists := m.GetSession(sessionID); exists {
		ctx.mu.Lock()
		ctx.IsBlocked = true
		ctx.mu.Unlock()
		m.UpdateSession(sessionID, ctx)
		m.logger.Warn("Blocked session", zap.String("sessionId", sessionID))
	}
}

// UnblockSession unblocks a session
func (m *Manager) UnblockSession(sessionID string) {
	if ctx, exists := m.GetSession(sessionID); exists {
		ctx.mu.Lock()
		ctx.IsBlocked = false
		ctx.mu.Unlock()
		m.UpdateSession(sessionID, ctx)
		m.logger.Info("Unblocked session", zap.String("sessionId", sessionID))
	}
}

// IsSessionBlocked checks if a session is blocked
func (m *Manager) IsSessionBlocked(sessionID string) bool {
	if ctx, exists := m.GetSession(sessionID); exists {
		ctx.mu.RLock()
		defer ctx.mu.RUnlock()
		return ctx.IsBlocked
	}
	return false
}

// CheckRateLimit checks if a session has exceeded the rate limit
func (m *Manager) CheckRateLimit(sessionID string) bool {
	ctx, exists := m.GetSession(sessionID)
	if !exists {
		return true // Allow if session doesn't exist
	}

	ctx.mu.Lock()
	defer ctx.mu.Unlock()

	now := time.Now()

	// Reset window if it's been more than windowSize
	if now.Sub(ctx.WindowStart) > m.windowSize {
		ctx.RequestCount = 0
		ctx.WindowStart = now
	}

	// Check if rate limit exceeded
	if ctx.RequestCount >= int64(m.requestsPerMinute) {
		m.logger.Warn("Rate limit exceeded",
			zap.String("sessionId", sessionID),
			zap.Int64("requestCount", ctx.RequestCount),
			zap.Int("limit", m.requestsPerMinute))
		return false
	}

	// Increment request count
	ctx.RequestCount++

	return true
}

// GetSessionStats returns session statistics
func (m *Manager) GetSessionStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_sessions":      m.cache.ItemCount(),
		"max_sessions":        m.maxSessions,
		"default_expiration":  m.defaultExpiration.String(),
		"cleanup_interval":    m.cleanupInterval.String(),
		"requests_per_minute": m.requestsPerMinute,
	}

	return stats
}

// GetActiveSessions returns information about active sessions
func (m *Manager) GetActiveSessions() []map[string]interface{} {
	var sessions []map[string]interface{}

	for sessionID, item := range m.cache.Items() {
		if ctx, ok := item.Object.(*Context); ok {
			ctx.mu.RLock()
			sessionInfo := map[string]interface{}{
				"id":            sessionID,
				"created_at":    ctx.CreatedAt,
				"last_accessed": ctx.LastAccessed,
				"call_count":    ctx.CallCount,
				"user_agent":    ctx.UserAgent,
				"remote_addr":   ctx.RemoteAddr,
				"is_blocked":    ctx.IsBlocked,
				"request_count": ctx.RequestCount,
			}
			ctx.mu.RUnlock()
			sessions = append(sessions, sessionInfo)
		}
	}

	return sessions
}

// cleanup removes expired sessions
func (m *Manager) cleanup() {
	m.cache.DeleteExpired()
	m.logger.Debug("Cleaned up expired sessions")
}

// generateSessionID generates a cryptographically secure session ID
func (m *Manager) generateSessionID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID if random generation fails
		return fmt.Sprintf("session_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// Close closes the session manager
func (m *Manager) Close() error {
	m.cache.Flush()
	m.logger.Info("Session manager closed")
	return nil
}

// Context methods

// UpdateLastAccessed updates the last accessed time
func (ctx *Context) UpdateLastAccessed() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.LastAccessed = time.Now()
}

// IncrementCallCount increments the call count
func (ctx *Context) IncrementCallCount() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.CallCount++
}

// GetCallCount returns the call count
func (ctx *Context) GetCallCount() int64 {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.CallCount
}

// IsExpired checks if the session is expired
func (ctx *Context) IsExpired(expiration time.Duration) bool {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return time.Since(ctx.LastAccessed) > expiration
}

// GetAge returns the age of the session
func (ctx *Context) GetAge() time.Duration {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return time.Since(ctx.CreatedAt)
}

// GetTimeSinceLastAccess returns the time since last access
func (ctx *Context) GetTimeSinceLastAccess() time.Duration {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return time.Since(ctx.LastAccessed)
}

// GetHeader returns a header value
func (ctx *Context) GetHeader(key string) string {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()
	return ctx.Headers[key]
}

// SetHeader sets a header value
func (ctx *Context) SetHeader(key, value string) {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	if ctx.Headers == nil {
		ctx.Headers = make(map[string]string)
	}
	ctx.Headers[key] = value
}

// GetInfo returns session information
func (ctx *Context) GetInfo() map[string]interface{} {
	ctx.mu.RLock()
	defer ctx.mu.RUnlock()

	return map[string]interface{}{
		"id":            ctx.ID,
		"created_at":    ctx.CreatedAt,
		"last_accessed": ctx.LastAccessed,
		"call_count":    ctx.CallCount,
		"user_agent":    ctx.UserAgent,
		"remote_addr":   ctx.RemoteAddr,
		"age":           time.Since(ctx.CreatedAt),
		"idle_time":     time.Since(ctx.LastAccessed),
		"is_blocked":    ctx.IsBlocked,
	}
}
