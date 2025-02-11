package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"

	"seer/config"
	"seer/interfaces"
	"seer/model"
)

var (
	LastProcessed = "LastProcessed"
	BlockNumKey   = "block"
	EventNumKey   = "event"
)

type handler struct {
	cfg         config.InfluxDBConfig
	client      influxdb2.Client
	writer      api.WriteAPIBlocking
	asyncWriter api.WriteAPI
	reader      api.QueryAPI
}

func (h *handler) getOrg() *domain.Organization {
	orgAPI := h.client.OrganizationsAPI()
	org, err := orgAPI.FindOrganizationByName(context.Background(), h.cfg.Org)
	if err == nil {
		return org
	}
	slog.Info("configured org not found, creating new one", "error ", err)
	//TODO: create doesn't work with org specific token
	_, err = orgAPI.CreateOrganizationWithName(context.Background(), h.cfg.Org)
	if err != nil {
		panic(fmt.Sprintf("Failed to create org error %s", err))
	}
	slog.Info("Successfully created org", "name", h.cfg.Org)
	return nil
}

func (h *handler) ensureBucket() {
	org := h.getOrg()
	bucketAPI := h.client.BucketsAPI()
	buckets, _ := bucketAPI.FindBucketsByOrgName(context.Background(), h.cfg.Org)
	if buckets != nil {
		for _, bucket := range *buckets {
			if bucket.Name == h.cfg.Bucket {
				return
			}
		}
	}

	slog.Info("configured bucket not found, creating new one")
	_, err := bucketAPI.CreateBucketWithName(context.Background(), org, h.cfg.Bucket, domain.RetentionRule{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create bucket error %s", err))
	}
	slog.Info("Successfully created bucket", "name", h.cfg.Bucket)
	return
}

func NewHandler(dbConfig config.InfluxDBConfig) interfaces.DatabaseHandler {
	slog.Info("connecting to DB", "url", dbConfig.URL)
	h := &handler{cfg: dbConfig}
	client := influxdb2.NewClient(dbConfig.URL, dbConfig.Token)
	h.reader = client.QueryAPI(h.cfg.Org)
	h.writer = client.WriteAPIBlocking(h.cfg.Org, h.cfg.Bucket)
	h.asyncWriter = client.WriteAPI(h.cfg.Org, h.cfg.Bucket)
	h.client = client
	h.ensureBucket()
	return h
}

func (h *handler) LastProcessed(fieldKey string) uint64 {
	query := fmt.Sprintf(`
			from(bucket:"%s")
			|> range(start: 0)
			|> filter(fn: (r) => r["_measurement"]=="%s")
			|> filter(fn: (r) => r["_field"]=="%s")
			|> sort(columns: ["_time"], desc: true)
			|> limit(n: 1)
		`, h.cfg.Bucket, LastProcessed, fieldKey)
	result, err := h.reader.Query(context.Background(), query)
	if err != nil {
		slog.Error("Unable to fetch last processed", "key", fieldKey, "error", err)
		return 0
	}
	for result.Next() {
		if value, ok := result.Record().Value().(uint64); ok {
			slog.Debug("Last processed", "key", fieldKey, "number", value)
			return value
		}
	}
	slog.Debug("can't find last processed", "key", fieldKey, "result", result)
	return 0
}

func (h *handler) SaveLastProcessed(key string, lp uint64) {
	point := influxdb2.NewPoint(LastProcessed,
		nil,
		map[string]interface{}{
			key: lp,
		},
		time.Now(),
	)
	h.asyncWriter.WritePoint(point)
	h.asyncWriter.Flush()
}

func (h *handler) WriteEventBlocking(schema model.EventSchema, timeStamp time.Time) error {
	point := influxdb2.NewPoint(schema.Measurement, schema.Tags, schema.Fields, timeStamp)
	err := h.writer.WritePoint(context.Background(), point)
	if err != nil {
		return err
	}
	return nil
}

func (h *handler) WriteEvent(schema model.EventSchema, timeStamp time.Time) {
	point := influxdb2.NewPoint(schema.Measurement, schema.Tags, schema.Fields, timeStamp)
	h.asyncWriter.WritePoint(point)
}

func (h *handler) WritePoint(measurement string, tags map[string]string, fields map[string]interface{}, ts time.Time) {
	point := influxdb2.NewPoint(measurement, tags, fields, ts)
	h.asyncWriter.WritePoint(point)
}

func (h *handler) Flush() {
	h.asyncWriter.Flush()
}

func (h *handler) Close() {
	h.client.Close()
}
