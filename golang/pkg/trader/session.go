package trader

import (
	"fmt"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
)

// SessionManager manages trading sessions
// 对应 tbsrc 控制文件中的交易时段管理
type SessionManager struct {
	config   *config.SessionConfig
	location *time.Location
}

// NewSessionManager creates a new session manager
func NewSessionManager(config *config.SessionConfig) *SessionManager {
	// Load timezone
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		// Default to UTC if timezone loading fails
		location = time.UTC
	}

	return &SessionManager{
		config:   config,
		location: location,
	}
}

// IsInSession returns whether current time is within trading session
func (sm *SessionManager) IsInSession() bool {
	now := time.Now().In(sm.location)

	// If no start/end time configured, always in session
	if sm.config.StartTime == "" || sm.config.EndTime == "" {
		return true
	}

	startTime, err := sm.parseTime(sm.config.StartTime, now)
	if err != nil {
		return true // Default to in session if parsing fails
	}

	endTime, err := sm.parseTime(sm.config.EndTime, now)
	if err != nil {
		return true
	}

	// Handle overnight sessions (e.g., 21:00 - 02:30)
	if endTime.Before(startTime) {
		// Overnight session
		return now.After(startTime) || now.Before(endTime)
	}

	// Normal session
	return now.After(startTime) && now.Before(endTime)
}

// GetNextSessionStart returns the time when the next session starts
func (sm *SessionManager) GetNextSessionStart() (time.Time, error) {
	now := time.Now().In(sm.location)

	if sm.config.StartTime == "" {
		return time.Time{}, fmt.Errorf("no start time configured")
	}

	startTime, err := sm.parseTime(sm.config.StartTime, now)
	if err != nil {
		return time.Time{}, err
	}

	// If current time is before today's start time, return today's start time
	if now.Before(startTime) {
		return startTime, nil
	}

	// Otherwise, return tomorrow's start time
	tomorrow := now.AddDate(0, 0, 1)
	return sm.parseTime(sm.config.StartTime, tomorrow)
}

// GetCurrentSessionEnd returns the time when the current session ends
func (sm *SessionManager) GetCurrentSessionEnd() (time.Time, error) {
	now := time.Now().In(sm.location)

	if sm.config.EndTime == "" {
		return time.Time{}, fmt.Errorf("no end time configured")
	}

	endTime, err := sm.parseTime(sm.config.EndTime, now)
	if err != nil {
		return time.Time{}, err
	}

	startTime, _ := sm.parseTime(sm.config.StartTime, now)

	// Handle overnight sessions
	if endTime.Before(startTime) && now.After(startTime) {
		// We're in an overnight session, end time is tomorrow
		tomorrow := now.AddDate(0, 0, 1)
		return sm.parseTime(sm.config.EndTime, tomorrow)
	}

	return endTime, nil
}

// GetTimeUntilSessionStart returns duration until next session starts
func (sm *SessionManager) GetTimeUntilSessionStart() time.Duration {
	nextStart, err := sm.GetNextSessionStart()
	if err != nil {
		return 0
	}

	now := time.Now().In(sm.location)
	if nextStart.Before(now) {
		return 0
	}

	return nextStart.Sub(now)
}

// GetTimeUntilSessionEnd returns duration until current session ends
func (sm *SessionManager) GetTimeUntilSessionEnd() time.Duration {
	sessionEnd, err := sm.GetCurrentSessionEnd()
	if err != nil {
		return 0
	}

	now := time.Now().In(sm.location)
	if sessionEnd.Before(now) {
		return 0
	}

	return sessionEnd.Sub(now)
}

// parseTime parses a time string (HH:MM:SS) and returns a time.Time for the given date
func (sm *SessionManager) parseTime(timeStr string, date time.Time) (time.Time, error) {
	// Parse time in format HH:MM:SS or HH:MM
	var hour, minute, second int
	var err error

	// Try HH:MM:SS format first
	_, err = fmt.Sscanf(timeStr, "%d:%d:%d", &hour, &minute, &second)
	if err != nil {
		// Try HH:MM format
		_, err = fmt.Sscanf(timeStr, "%d:%d", &hour, &minute)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid time format: %s (expected HH:MM:SS or HH:MM)", timeStr)
		}
		second = 0
	}

	// Create time for the given date
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, 0, sm.location), nil
}

// GetSessionInfo returns information about the current session
func (sm *SessionManager) GetSessionInfo() map[string]interface{} {
	inSession := sm.IsInSession()

	info := map[string]interface{}{
		"in_session": inSession,
		"timezone":   sm.config.Timezone,
		"auto_start": sm.config.AutoStart,
		"auto_stop":  sm.config.AutoStop,
	}

	if inSession {
		timeUntilEnd := sm.GetTimeUntilSessionEnd()
		info["time_until_end"] = timeUntilEnd.String()
	} else {
		timeUntilStart := sm.GetTimeUntilSessionStart()
		info["time_until_start"] = timeUntilStart.String()
	}

	return info
}
