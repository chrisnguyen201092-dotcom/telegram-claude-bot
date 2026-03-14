package events

import (
	"reflect"
	"sync"
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

type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]func(EventData)
}

func NewEventBus() *EventBus {
	return &EventBus{
		listeners: make(map[string][]func(EventData)),
	}
}

func (eb *EventBus) Emit(eventType string, data map[string]any) {
	event := EventData{
		Type:      eventType,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	eb.mu.RLock()
	specific := make([]func(EventData), len(eb.listeners[eventType]))
	copy(specific, eb.listeners[eventType])
	wildcard := make([]func(EventData), len(eb.listeners["*"]))
	copy(wildcard, eb.listeners["*"])
	eb.mu.RUnlock()

	for _, cb := range specific {
		cb := cb
		go cb(event)
	}
	for _, cb := range wildcard {
		cb := cb
		go cb(event)
	}
}

func (eb *EventBus) On(eventType string, callback func(EventData)) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.listeners[eventType] = append(eb.listeners[eventType], callback)
}

func (eb *EventBus) Off(eventType string, callback func(EventData)) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	cbs := eb.listeners[eventType]
	cbPtr := reflect.ValueOf(callback).Pointer()
	result := cbs[:0]
	for _, cb := range cbs {
		if reflect.ValueOf(cb).Pointer() != cbPtr {
			result = append(result, cb)
		}
	}
	eb.listeners[eventType] = result
}

var Bus = NewEventBus()
