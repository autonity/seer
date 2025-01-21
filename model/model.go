package model

//EventSchema is the combination of abi schema and influx db schema, inf future this could be decoupled into separate event objectsg
type EventSchema struct {
	Measurement string                 `json:"event_name"`
	Fields      map[string]interface{} `json:"fields"`
	Tags        map[string]string
}
