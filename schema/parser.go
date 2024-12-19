package schema

import (
	"bytes"
	"log/slog"
	"os"

	"github.com/ethereum/go-ethereum/accounts/abi"

	"Seer/interfaces"
	"Seer/model"
)

type abiParser struct {
	schemas []model.EventSchema
}

func NewABIParser() interfaces.ABIParser {
	return &abiParser{}
}

func (ap *abiParser) Parse(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		slog.Error("Error opening abi file", "error", err)
		return err
	}
	byteReader := bytes.NewReader(data)
	parsedABI, err := abi.JSON(byteReader)
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
		ap.schemas = append(ap.schemas, schema)
	}
	return nil
}

func (ap *abiParser) EventSchema(eventName string) model.EventSchema {
	for _, schema := range ap.schemas {
		if schema.Name == eventName {
			return schema
		}
	}
	return model.EventSchema{}
}
