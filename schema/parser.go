package schema

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/autonity/autonity/accounts/abi"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"Seer/config"
	"Seer/events/registry"
	"Seer/interfaces"
	"Seer/model"
)

type EventDetails struct {
	sync.RWMutex
	data map[common.Hash]struct {
		abi    abi.Event
		schema model.EventSchema
	}
}

type EventMeta struct {
	sync.RWMutex
	data map[string]struct {
		index int
		ids   []common.Hash
	}
}

type abiParser struct {
	cfg         config.ABIConfig
	eventDetail EventDetails // id to abi
	eventMeta   EventMeta    // event Name to index
	dbHandler   interfaces.DatabaseHandler
}

func NewABIParser(cfg config.ABIConfig, dh interfaces.DatabaseHandler) interfaces.ABIParser {
	return &abiParser{
		cfg: cfg,
		eventDetail: EventDetails{
			sync.RWMutex{},
			make(map[common.Hash]struct {
				abi    abi.Event
				schema model.EventSchema
			}),
		},
		eventMeta: EventMeta{
			sync.RWMutex{},
			make(map[string]struct {
				index int
				ids   []common.Hash
			}),
		},
		dbHandler: dh,
	}
}

func (ap *abiParser) LoadABIS() error {
	slog.Info("Reading ABIs from path", "dir", ap.cfg.Dir)
	files, err := os.ReadDir(ap.cfg.Dir)
	if err != nil {
		slog.Error("error reading dir", "name", ap.cfg.Dir, "error", err)
		return err
	}
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".abi" {
			absPath := filepath.Join(ap.cfg.Dir, file.Name())
			slog.Debug("Parsing...", "file", absPath)
			err := ap.Parse(absPath)
			if err != nil {
				return err
			}
		}
	}
	ap.ListEvents()
	return nil
}

func (ap *abiParser) ListEvents() {
	names := make([]string, 0)
	ap.eventDetail.RLock()
	defer ap.eventDetail.RUnlock()

	for _, ev := range ap.eventDetail.data {
		names = append(names, ev.schema.Measurement)
	}

	slog.Debug("All events", "events", names)
}

func (ap *abiParser) Start() error {
	err := ap.LoadABIS()
	if err != nil {
		return err
	}
	go WatchNewABIs(ap.cfg.Dir, ap)
	return nil
}

func (ap *abiParser) Stop() error {
	//TODO: stop sequence
	return nil
}

func (ap *abiParser) Decode(log types.Log) (model.EventSchema, error) {
	slog.Debug("event Decode", "topic-0", log.Topics[0])

	ap.eventDetail.RLock()
	defer ap.eventDetail.RUnlock()
	eventDetails := ap.eventDetail.data[log.Topics[0]]
	decodedEvent := map[string]interface{}{}
	indexed := make([]abi.Argument, 0)
	for _, input := range eventDetails.abi.Inputs {
		if input.Indexed {
			indexed = append(indexed, input)
		}
	}
	if len(indexed) > 0 {
		err := abi.ParseTopicsIntoMap(decodedEvent, indexed, log.Topics[1:])
		if err != nil {
			slog.Error("unable to decode indexed events", "error", err)
			return model.EventSchema{}, err
		}
	}

	err := eventDetails.abi.Inputs.UnpackIntoMap(decodedEvent, log.Data)
	if err != nil {
		slog.Error("unable to decode event", "error", err)
		return model.EventSchema{}, err
	}
	evSchema := model.EventSchema{Measurement: eventDetails.abi.Name, Fields: decodedEvent}
	return evSchema, nil
}

func (ap *abiParser) getEventName(id common.Hash, name string) string {
	ap.eventMeta.Lock()
	defer ap.eventMeta.Unlock()
	evMeta, ok := ap.eventMeta.data[name]
	if !ok {
		return name
	}
	for _, evID := range evMeta.ids {
		if evID == id {
			// we have the event
			return name
		}
	}

	evMeta.index += 1
	ap.eventMeta.data[name] = evMeta
	return fmt.Sprintf("%s_u%d", name, evMeta.index)
}

func (ap *abiParser) Parse(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		slog.Error("Error opening abi file", "error", err)
		return err
	}
	parsedABI, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		slog.Error("Error reading abi file", "error", err)
		return err
	}

	ap.eventDetail.Lock()
	defer ap.eventDetail.Unlock()
	// append schemas
	for name, event := range parsedABI.Events {
		schema := model.EventSchema{
			Measurement: ap.getEventName(event.ID, name),
			Fields:      map[string]interface{}{},
		}
		for _, input := range event.Inputs {
			fieldType := input.Type.String()
			schema.Fields[input.Name] = fieldType
		}
		registry.RegisterHandler()
		ap.eventDetail.data[event.ID] = struct {
			abi    abi.Event
			schema model.EventSchema
		}{
			abi:    event,
			schema: schema,
		}
	}
	return nil
}
