package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common/fixsizecache"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/crypto"
	"github.com/spaolacci/murmur3"

	"seer/helper"
	"seer/interfaces"
	"seer/model"
	"seer/net"
)

type Location struct {
	Lat float64
	Lon float64
}

type CacheEntry struct {
	Location  Location
	Timestamp time.Time
}

var (
	locationCache           *fixsizecache.Cache[string, CacheEntry]
	cacheTTL                = time.Minute * 10
	ipInfoSource            = "http://ip-api.com/json/"
	nodeLocationMeasurement = "nodeLocation"
)

func HashString(ip string) uint {
	return uint(murmur3.Sum64([]byte(ip)))
}

func init() {
	locationCache = fixsizecache.New[string, CacheEntry](1000, 2, HashString)
}

func getCoordinates(ip string) (float64, float64, error) {
	// Check cache first
	e, exists := locationCache.Get(ip)

	// If in cache and not expired, return cached value
	if exists {
		entry := e.(CacheEntry)
		if time.Since(entry.Timestamp) < cacheTTL {
			return entry.Location.Lat, entry.Location.Lon, nil
		}
	}
	slog.Info("Location info not found for", "ip", ip, "fetch from source", ipInfoSource)

	resp, err := http.Get(ipInfoSource + ip)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch IP location: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		Status string  `json:"status"`
		Lat    float64 `json:"lat"`
		Lon    float64 `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, fmt.Errorf("failed to decode response: %v", err)
	}
	if data.Status != "success" {
		return 0, 0, fmt.Errorf("API error for IP %s", ip)
	}

	locationCache.Add(ip, CacheEntry{
		Location:  Location{Lat: data.Lat, Lon: data.Lon},
		Timestamp: time.Now(),
	})

	return data.Lat, data.Lon, nil
}

// EventName + Handler

type NewEpochHandler struct {
	DBHandler interfaces.DatabaseHandler
}

func (ev *NewEpochHandler) Handle(schema model.EventSchema, header *types.Header, cp net.ConnectionProvider) {
	if header.Epoch == nil {
		slog.Error("NewEpoch Handler, committee information is nor present")
		return
	}
	//TODO
	con := cp.GetWebSocketConnection()
	autBindings, err := autonity.NewAutonity(helper.AutonityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
	}

	enodeStrings, err := autBindings.GetCommitteeEnodes(&bind.CallOpts{
		BlockNumber: header.Number,
	})

	totalVotingPower, err := autBindings.EpochTotalBondedStake(&bind.CallOpts{
		BlockNumber: header.Number,
	})
	//totalVotingPower := header.Epoch.Committee.TotalVotingPower()

	nodes := types.NewNodes(enodeStrings, true)
	fields := map[string]interface{}{}
	tags := map[string]string{}
	ts := time.Unix(int64(header.Time), 0)

	for _, node := range nodes.List {
		lat, lon, err := getCoordinates(node.IP().String())
		if err != nil {
			slog.Error("error fetching latitude longitude", "error", err)
			continue
		}
		addr := crypto.PubkeyToAddress(*node.Pubkey())
		member := header.Epoch.Committee.MemberByAddress(addr)
		tags["address"] = addr.String()
		fields["nodeStake"] = member.VotingPower.Uint64()
		fields["percentageStake"] = (member.VotingPower.Uint64() / totalVotingPower.Uint64()) * 100
		fields["lat"] = lat
		fields["lon"] = lon
		fields["ip"] = node.IP().String()
		fields["block"] = header.Number.Uint64()
		ev.DBHandler.WritePoint(nodeLocationMeasurement, tags, fields, ts)
	}
}
