package model

type EventSchema struct {
	Name   string                 `json:"event_name"`
	Fields map[string]interface{} `json:"fields"`
}
