package server

import (
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"
)

type EventType string

const (
	EventImplantCheckin    EventType = "implant.checkin"
	EventImplantDisconnect EventType = "implant.disconnect"
	EventTaskComplete      EventType = "task.complete"
	EventTaskFail          EventType = "task.fail"
	EventCredentialFound   EventType = "credential.found"
	EventAlertTriggered    EventType = "alert.triggered"
)

type Event struct {
	Type      EventType
	AgentID   string
	AgentHost string
	Timestamp time.Time
	Data      map[string]interface{}
}

type EventHandler func(Event)

type EventManager struct {
	mu       sync.RWMutex
	handlers map[EventType][]EventHandler
	db       *gorm.DB
}

func NewEventManager(database *gorm.DB) *EventManager {
	return &EventManager{
		handlers: make(map[EventType][]EventHandler),
		db:       database,
	}
}

func (em *EventManager) On(et EventType, handler EventHandler) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.handlers[et] = append(em.handlers[et], handler)
}

func (em *EventManager) Emit(evt Event) {
	em.mu.RLock()
	handlers := make([]EventHandler, len(em.handlers[evt.Type]))
	copy(handlers, em.handlers[evt.Type])
	em.mu.RUnlock()

	for _, h := range handlers {
		go func(handler EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("event handler panic", "panic", r)
				}
			}()
			handler(evt)
		}(h)
	}
}
