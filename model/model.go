package model

type EventSchema struct {
	Measurement string                 `json:"event_name"`
	Fields      map[string]interface{} `json:"fields"`
}
