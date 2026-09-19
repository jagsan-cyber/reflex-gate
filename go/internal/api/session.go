package api

import (
	"sync"
	"time"
)

// SessionState tracks per-session slot binding and cache history
type SessionState struct {
	SessionID     string
	SlotID        int
	LastPromptLen int
	TurnCount     int
	LastActive    time.Time
}

// SessionManager binds sessions to llama-server slots for prefix cache reuse
type SessionManager struct {
	mu       sync.Mutex
	sessions map[string]*SessionState
	numSlots int
	nextSlot int
}

// NewSessionManager creates a session manager
func NewSessionManager(numSlots int) *SessionManager {
	if numSlots <= 0 {
		numSlots = 1
	}
	return &SessionManager{
		sessions: make(map[string]*SessionState),
		numSlots: numSlots,
		nextSlot: 0,
	}
}

// SetNumSlots updates available slot count from llama-server
func (sm *SessionManager) SetNumSlots(n int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if n > 0 {
		sm.numSlots = n
	}
}

// GetSlot returns the bound slot ID for a session
func (sm *SessionManager) GetSlot(sessionID string, defaultSlot int) int {
	if sessionID == "" {
		return defaultSlot
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if st, ok := sm.sessions[sessionID]; ok {
		st.LastActive = time.Now()
		return st.SlotID
	}

	slot := defaultSlot
	if sm.numSlots > 1 {
		slot = sm.nextSlot % sm.numSlots
		sm.nextSlot++
	}

	sm.sessions[sessionID] = &SessionState{
		SessionID:  sessionID,
		SlotID:     slot,
		LastActive: time.Now(),
	}
	return slot
}

// RecordTurn updates turn stats for incremental detection
func (sm *SessionManager) RecordTurn(sessionID string, slotID, promptLen int) {
	if sessionID == "" {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	st, ok := sm.sessions[sessionID]
	if !ok {
		st = &SessionState{SessionID: sessionID, SlotID: slotID}
		sm.sessions[sessionID] = st
	}
	st.SlotID = slotID
	st.LastPromptLen = promptLen
	st.TurnCount++
	st.LastActive = time.Now()
}

// IsIncremental returns true if this session has a previous cache entry
func (sm *SessionManager) IsIncremental(sessionID string, promptLen int) bool {
	if sessionID == "" {
		return false
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	st, ok := sm.sessions[sessionID]
	if !ok {
		return false
	}
	return st.TurnCount > 0 && promptLen >= st.LastPromptLen
}
