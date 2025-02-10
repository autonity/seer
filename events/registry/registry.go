package registry

import (
	"sync"

	"seer/events/handlers"
	"seer/interfaces"
)

var (
	handlerRegistry = make(map[string]interfaces.EventHandler)
	mu              sync.RWMutex
)

const (
	NewEpoch               = "NewEpoch"
	InactivityJailingEvent = "InactivityJailingEvent"
	SlashingEvent          = "SlashingEvent"
	Penalized              = "Penalized"
)

func RegisterEventHandlers(dbHandler interfaces.DatabaseHandler) {
	mu.Lock()
	defer mu.Unlock()

	handlerRegistry[NewEpoch] = &handlers.NewEpochHandler{DBHandler: dbHandler}
	handlerRegistry[InactivityJailingEvent] = &handlers.InactivityjJaillingEventHandler{DBHandler: dbHandler}
	handlerRegistry[Penalized] = &handlers.PenalizedHandler{DBHandler: dbHandler}
	handlerRegistry[SlashingEvent] = &handlers.SlashingEventHandler{DBHandler: dbHandler}
}

func GetEventHandler(eventName string) interfaces.EventHandler {
	mu.RLock()
	defer mu.RUnlock()
	return handlerRegistry[eventName]
}
