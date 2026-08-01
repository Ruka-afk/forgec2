package server

import (
	"context"
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
	_                      EventType = "alert.triggered" // reserved
)

type Event struct {
	Type      EventType
	AgentID   string
	AgentHost string
	AgentIP   string
	AgentOS   string
	User      string
	Timestamp time.Time
	Data      map[string]interface{}
}

type EventHandler func(Event)

const maxConcurrentEventHandlers = 32
const eventQueueSize = 256

type EventManager struct {
	mu         sync.RWMutex
	handlers   map[EventType][]EventHandler
	db         *gorm.DB
	workerSem  chan struct{}
	eventQueue chan Event
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewEventManager(database *gorm.DB) *EventManager {
	ctx, cancel := context.WithCancel(context.Background())
	em := &EventManager{
		handlers:   make(map[EventType][]EventHandler),
		db:         database,
		workerSem:  make(chan struct{}, maxConcurrentEventHandlers),
		eventQueue: make(chan Event, eventQueueSize),
		ctx:        ctx,
		cancel:     cancel,
	}
	go em.queueReader()
	return em
}

func (em *EventManager) Shutdown() {
	em.cancel()
}

func (em *EventManager) On(et EventType, handler EventHandler) {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.handlers[et] = append(em.handlers[et], handler)
}

func (em *EventManager) Emit(evt Event) {
	select {
	case <-em.ctx.Done():
		return
	case em.eventQueue <- evt:
	default:
		slog.Warn("Event queue full, dropping event", "type", evt.Type)
	}
}

func (em *EventManager) queueReader() {
	for {
		select {
		case <-em.ctx.Done():
			return
		case evt := <-em.eventQueue:
			em.mu.RLock()
			handlers := make([]EventHandler, len(em.handlers[evt.Type]))
			copy(handlers, em.handlers[evt.Type])
			em.mu.RUnlock()

			for _, h := range handlers {
				select {
				case <-em.ctx.Done():
					return
				case em.workerSem <- struct{}{}:
					go func(handler EventHandler) {
						defer func() {
							<-em.workerSem
							if r := recover(); r != nil {
								slog.Error("Event handler panic", "panic", r)
							}
						}()
						handler(evt)
					}(h)
				default:
					slog.Warn("Event handler worker saturated, dropping event", "type", evt.Type)
				}
			}
		}
	}
}
