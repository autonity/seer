package registry

import (
	"sync"

	"seer/events/handlers"
	"seer/interfaces"
)

var (
	handlerRegistry = make(map[string]handlers.EventHandler)
	mu              sync.RWMutex
)

const (
	NewEpoch               = "NewEpoch"
	InactivityJailingEvent = "InactivityJailingEvent"
	SlashingEvent          = "SlashingEvent"
	Penalized              = "Penalized"
	NewRound               = "NewRound"
	NewAccusation          = "NewAccusation"
	NewFaultProof          = "NewFaultProof"
)

func RegisterEventHandlers(dbHandler interfaces.DatabaseHandler) {
	mu.Lock()
	defer mu.Unlock()

	handlerRegistry[NewEpoch] = &handlers.NewEpochHandler{DBHandler: dbHandler}
	handlerRegistry[InactivityJailingEvent] = &handlers.InactivityjJaillingEventHandler{DBHandler: dbHandler}
	handlerRegistry[Penalized] = &handlers.PenalizedHandler{DBHandler: dbHandler}
	handlerRegistry[SlashingEvent] = &handlers.SlashingEventHandler{DBHandler: dbHandler}
	handlerRegistry[NewRound] = &handlers.NewRoundHandler{DBHandler: dbHandler}
	handlerRegistry[NewAccusation] = &handlers.NewAccusationHandler{DBHandler: dbHandler}
	handlerRegistry[NewFaultProof] = &handlers.NewFaultProofHandler{DBHandler: dbHandler}
}

func GetEventHandler(eventName string) handlers.EventHandler {
	mu.RLock()
	defer mu.RUnlock()
	return handlerRegistry[eventName]
}
