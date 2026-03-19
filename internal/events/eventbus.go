package events

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	EventMessageReceived  = "message_received"
	EventMessageSent      = "message_sent"
	EventSDKStart         = "sdk_start"
	EventSDKComplete      = "sdk_complete"
	EventSDKError         = "sdk_error"
	EventUserJoined       = "user_joined"
	EventLogAdded         = "log_added"
	EventSettingChanged   = "setting_changed"
	EventRuleChanged      = "rule_changed"
	EventMemoryUpdated    = "memory_updated"
	EventSessionChanged   = "session_changed"
	EventMCPChanged       = "mcp_changed"
	EventConfigChanged    = "config_changed"
	EventSessionCompacted = "session_compacted"
	EventSDKToolUse       = "sdk_tool_use"
	EventSDKToolResult    = "sdk_tool_result"
	EventSDKThinking      = "sdk_thinking"
	EventSDKAskQuestion   = "sdk_ask_question"
	EventSDKAskAnswered   = "sdk_ask_answered"
	EventSDKPlan          = "sdk_plan"
)

type EventData struct {
	Type      string         `json:"type"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data,omitempty"`
}

// M5: subscription wraps a callback with a unique ID for reliable removal.
type subscription struct {
	id       int
	callback func(EventData)
}

// M5: nextSubID is an atomic counter for generating unique subscription IDs.
var nextSubID atomic.Int64

type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]subscription
}

func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]subscription),
	}
}

func (eb *EventBus) Emit(eventType string, data map[string]any) {
	event := EventData{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	eb.mu.RLock()
	specific := make([]subscription, len(eb.listeners[eventType]))
	copy(specific, eb.listeners[eventType])
	wildcard := make([]subscription, len(eb.listeners["*"]))
	copy(wildcard, eb.listeners["*"])
	eb.mu.RUnlock()

	// M10: Call listeners synchronously — they're fast log/broadcast ops.
	// Avoids unbounded goroutine spawning.
	for _, sub := range specific {
		sub.callback(event)
	}
	for _, sub := range wildcard {
		sub.callback(event)
	}
}

// M5: On registers a listener and returns a subscription ID for reliable removal via Off.
func (eb *EventBus) On(eventType string, callback func(EventData)) int {
	id := int(nextSubID.Add(1))
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.listeners[eventType] = append(eb.listeners[eventType], subscription{id: id, callback: callback})
	return id
}

// M5: Off removes a listener by subscription ID (returned by On), avoiding pointer comparison issues.
func (eb *EventBus) Off(eventType string, subID int) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	subs := eb.listeners[eventType]
	result := subs[:0]
	for _, s := range subs {
		if s.id != subID {
			result = append(result, s)
		}
	}
	eb.listeners[eventType] = result
}

var Bus = NewEventBus()
