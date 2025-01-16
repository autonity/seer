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
	blockNumKey   = "block"
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
	slog.Info("configured org not found, creating new one")
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

func (h *handler) LastProcessed() uint64 {
	query := fmt.Sprintf(`
			from(bucket:"%s")
			|> range(start: 0)
			|> filter(fn: (r) => r["_measurement"]=="%s")
			|> sort(columns: ["_time"], desc: true)
			|> limit(n: 1)
		`, h.cfg.Bucket, LastProcessed)
	result, err := h.reader.Query(context.Background(), query)
	if err != nil {
		slog.Error("Unable to fetch last processed", "error", err)
		return 0
	}
	for result.Next() {
		if value, ok := result.Record().Value().(uint64); ok {
			slog.Debug("Last processed block", "number", value)
			return value
		}
	}
	slog.Debug("can't find last processed", "result", result)
	return 0
}

func (h *handler) SaveLastProcessed(lp uint64) {
	point := influxdb2.NewPoint(LastProcessed,
		nil,
		map[string]interface{}{
			blockNumKey: lp,
		},
		time.Now(),
	)
	h.asyncWriter.WritePoint(point)
	h.asyncWriter.Flush()
}

func (h *handler) WriteEventBlocking(schema model.EventSchema, tags map[string]string, timeStamp time.Time) error {
	point := influxdb2.NewPoint(schema.Measurement, tags, schema.Fields, timeStamp)
	err := h.writer.WritePoint(context.Background(), point)
	if err != nil {
		return err
	}
	return nil
}

func (h *handler) WriteEvent(schema model.EventSchema, tags map[string]string, timeStamp time.Time) {
	point := influxdb2.NewPoint(schema.Measurement, tags, schema.Fields, timeStamp)
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
