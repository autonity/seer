package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/domain"

	"Seer/config"
	"Seer/interfaces"
	"Seer/model"
)

type handler struct {
	cfg    config.InfluxDBConfig
	client influxdb2.Client
}

func (h *handler) getOrg() *domain.Organization {
	orgAPI := h.client.OrganizationsAPI()
	org, err := orgAPI.FindOrganizationByName(context.Background(), h.cfg.Org)
	if err == nil {
		return org
	}
	slog.Info("configured org not found, creating new one")
	_, err = orgAPI.CreateOrganizationWithName(context.Background(), h.cfg.Org)
	if err != nil {
		panic(fmt.Sprintf("Failed to create org error %s", err))
	}
	slog.Info("Successfully created org", "name", h.cfg.Org)
	return nil
}

func (h *handler) ensureBucket() {
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
	_, err := bucketAPI.CreateBucketWithName(context.Background(), h.getOrg(), h.cfg.Bucket, domain.RetentionRule{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create bucket error %s", err))
	}
	slog.Info("Successfully created bucket", "name", h.cfg.Bucket)
	return
}

func NewHandler(dbConfig config.InfluxDBConfig) interfaces.DatabaseHandler {
	slog.Info("connecting to DB", "url", dbConfig.URL)
	h := &handler{cfg: dbConfig}
	h.client = influxdb2.NewClient(dbConfig.URL, dbConfig.Token)
	h.ensureBucket()
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
