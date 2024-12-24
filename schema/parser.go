package schema

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/autonity/autonity/accounts/abi"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"

	"Seer/config"
	"Seer/interfaces"
	"Seer/model"
)

// TODO: insert event with contract tags
type EventDetails struct {
	abi    abi.Event
	schema model.EventSchema
}

type abiParser struct {
	cfg       config.ABIConfig
	evDetails map[common.Hash]EventDetails
	dbHandler interfaces.DatabaseHandler
}

func NewABIParser(cfg config.ABIConfig, dh interfaces.DatabaseHandler) interfaces.ABIParser {
	return &abiParser{
		cfg:       cfg,
		evDetails: make(map[common.Hash]EventDetails),
		dbHandler: dh,
	}
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
			slog.Info("Parsing...", "file", absPath)
			err := ap.Parse(absPath)
			if err != nil {
				return err
			}
		}
	}
	//TODO: review
	// write place-holder event
	slog.Info("writing events in DB")
	for _, evDetail := range ap.evDetails {
		ap.dbHandler.WriteEvent(evDetail.schema, nil)
	}
	slog.Info("wrote events in DB")
	return nil
}

func (ap *abiParser) DecodeEvent(log types.Log) {
	eventDetails := ap.evDetails[log.Topics[0]] // this should give the correctABI
	decodedEvent := map[string]interface{}{}
	err := eventDetails.abi.Inputs.UnpackIntoMap(decodedEvent, log.Data)
	if err != nil {
		slog.Error("unable to decode event", "error", "err")
		return
	}
	evSchema := model.EventSchema{Name: eventDetails.abi.Name, Fields: decodedEvent}
	//TODO: tags
	ap.dbHandler.WriteEvent(evSchema, nil)
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

	// append schemas
	for name, event := range parsedABI.Events {
		schema := model.EventSchema{
			Name:   name,
			Fields: map[string]interface{}{},
		}
		for _, input := range event.Inputs {
			fieldType := input.Type.String()
			schema.Fields[input.Name] = fieldType
		}
		ap.evDetails[event.ID] = EventDetails{abi: event, schema: schema}
	}
	return nil
}

func (ap *abiParser) EventSchema(eventName string) model.EventSchema {
	//TODO:
	return model.EventSchema{}
}
