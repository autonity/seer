package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity/bindings"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/common/fixsizecache"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/crypto"
	"github.com/spaolacci/murmur3"

	"seer/helper"
	"seer/interfaces"
	"seer/model"
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
	ValidatorToOracleMap    = "validatorToOracle"
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

func (ev *NewEpochHandler) Handle(schema model.EventSchema, header *types.Header, core interfaces.Core) {
	slog.Info("New Epoch Handler")
	if header.Epoch == nil {
		slog.Error("NewEpoch Handler, committee information is not present")
		return
	}
	con := core.ConnectionProvider().GetWebSocketConnection()
	autBindings, err := bindings.NewAutonity(helper.AutonityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return
	}

	enodeStrings, err := autBindings.GetCommitteeEnodes(&bind.CallOpts{
		BlockNumber: header.Number,
	})

	nodes := types.NewNodes(enodeStrings, true)
	fields := map[string]interface{}{}
	tags := map[string]string{}
	ts := time.Unix(int64(header.Time), 0)

	totalVotingPower, committee, err := ev.GetStake(autBindings, header)
	if err != nil {
		slog.Error("NewEpoch Handler, can't fetch stake", "error", err)
		return
	}
	totalVotingPowerFloat := new(big.Float).SetInt(totalVotingPower)
	for _, node := range nodes.List {
		lat, lon, err := getCoordinates(node.IP().String())
		if err != nil {
			slog.Error("error fetching latitude longitude", "error", err)
			continue
		}

		addr := crypto.PubkeyToAddress(*node.Pubkey())
		votingPower := getVotingPower(committee, addr)

		tags["address"] = addr.String()
		fields["nodeStake"] = votingPower
		fields["totalStake"] = totalVotingPower

		votingPowerFloat := new(big.Float).SetInt(votingPower)
		percentage := new(big.Float).Quo(votingPowerFloat, totalVotingPowerFloat)
		percentage.Mul(percentage, big.NewFloat(100))

		fields["percentageStake"] = percentage.Text('f', 2)
		fields["lat"] = lat
		fields["lon"] = lon
		fields["ip"] = node.IP().String()
		fields["block"] = header.Number.Uint64()
		ev.DBHandler.WritePoint(nodeLocationMeasurement, tags, fields, ts)
	}

	oracleBindings, err := bindings.NewOracle(helper.OracleContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
		return
	}

	oracleVoters, err := oracleBindings.GetNewVoters(&bind.CallOpts{
		BlockNumber: header.Number,
	})
	if err != nil {
		slog.Error("unable to fetch oracleVoters", "error", err)
		return

	}
	committee, err = autBindings.GetCommittee(&bind.CallOpts{
		BlockNumber: header.Number,
	})

	if err != nil {
		slog.Error("unable to fetch committee", "error", err)
		return
	}

	fields = make(map[string]interface{})
	tags = make(map[string]string)
	for i := range len(oracleVoters) {
		fields["oracleAddress"] = oracleVoters[i]
		fields["validatorAddress"] = committee[i].Addr
		fields["block"] = header.Number
		tags["index"] = strconv.Itoa(i)
		ev.DBHandler.WritePoint(ValidatorToOracleMap, tags, fields, ts)
	}
	slog.Info("Processed new Epoch")
}

func (ev *NewEpochHandler) GetStake(autBindings *bindings.Autonity, header *types.Header) (*big.Int, []bindings.IAutonityCommitteeMember, error) {
	totalVotingPower, err := autBindings.GetEpochTotalBondedStake(&bind.CallOpts{
		BlockNumber: header.Number,
	})
	if err != nil {
		return nil, nil, err
	}

	committee, err := autBindings.GetCommittee(&bind.CallOpts{
		BlockNumber: header.Number,
	})
	if err != nil {
		return nil, nil, err
	}
	return totalVotingPower, committee, nil
}

func getVotingPower(committee []bindings.IAutonityCommitteeMember, address common.Address) *big.Int {
	for _, c := range committee {
		if c.Addr == address {
			return c.VotingPower
		}
	}
	return nil
}
