package schema

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"Seer/config"
	"Seer/interfaces"
	"Seer/model"
)

// TODO: insert event with contract tags
type contractSchema struct {
	contractAddress common.Address
	schemas         []model.EventSchema
}

type abiParser struct {
	cfg       config.ABIConfig
	schemas   map[string]model.EventSchema
	dbHandler interfaces.DatabaseHandler
}

func NewABIParser(cfg config.ABIConfig, dh interfaces.DatabaseHandler) interfaces.ABIParser {
	return &abiParser{
		cfg:       cfg,
		schemas:   make(map[string]model.EventSchema),
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
			slog.Info("Parsing...", "file", file.Name())
			err := ap.Parse(file.Name())
			if err != nil {
				return err
			}
		}
	}
	//TODO: review
	// write place-holder event
	for _, schema := range ap.schemas {
		ap.dbHandler.WriteEvent(schema, nil)
	}
	return nil
}

func (ap *abiParser) DecodeEvent(log types.Log) {
	//TODO:

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
		ap.schemas[name] = schema
	}
	return nil
}

func (ap *abiParser) EventSchema(eventName string) model.EventSchema {
	return ap.schemas[eventName]
}
