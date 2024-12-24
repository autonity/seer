package db

import (
	"log/slog"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"

	"Seer/config"
	"Seer/interfaces"
	"Seer/model"
)

type handler struct {
	cfg    config.InfluxDBConfig
	client influxdb2.Client
}

func NewHandler(dbConfig config.InfluxDBConfig) interfaces.DatabaseHandler {
	slog.Info("connecting to DB", "url", dbConfig.URL)
	h := &handler{cfg: dbConfig}
	h.client = influxdb2.NewClient(dbConfig.URL, dbConfig.Token)
	return h
}

func (h *handler) WriteEvent(schema model.EventSchema, tags map[string]string) {
	writer := h.client.WriteAPI(h.cfg.Org, h.cfg.Bucket)
	//TODO: what else we need here
	point := influxdb2.NewPoint(schema.Name, tags, schema.Fields, time.Now())
	writer.WritePoint(point)
	writer.Flush()
}

func (h *handler) LastProcessed() int64 {
	return 0

}
func (h *handler) Close() {
	h.client.Close()
}
