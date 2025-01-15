package registry

import (
	"sync"

	"Seer/events/handlers"
	"Seer/interfaces"
)

var (
	handlerRegistry = make(map[string]interfaces.EventHandler)
	mu              sync.RWMutex
)

func RegisterHandler() {
	mu.Lock()
	defer mu.Unlock()

	handlerRegistry["NewEpoch"] = &handlers.NewEpochHandler{}
	handlerRegistry["InactivityJailingEvent"] = &handlers.InactivityjJaillingEventHandler{}
}

func GetHandler(eventName string) interfaces.EventHandler {
	mu.RLock()
	defer mu.RUnlock()
	return handlerRegistry[eventName]
}
