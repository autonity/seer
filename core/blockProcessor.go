package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/p2p"

	"seer/helper"
	"seer/interfaces"
)

var (
	acnPeers       = "ACNPeers"
	BlockHeader    = "BlockHeader"
	BlockTimestamp = "BlockTimestamp"
	QCAbsentees    = "QuorumCertificateAbsentees"
	APAbsentees    = "ActivityProofAbsentees"
)

type blockProcessor struct {
	core                 *core
	ctx                  context.Context
	headerCh             chan *types.Header
	blockRecorderFields  map[string]interface{}
	acnPeerFields        map[string]interface{}
	blockTimestampFields map[string]interface{}
	qcAbsenteesFields    map[string]interface{}
	apAbsenteesFields    map[string]interface{}
	qcTags               map[string]string
	apTags               map[string]string
}

func NewBlockProcessor(ctx context.Context, core *core, newBlocks chan *types.Header) interfaces.Processor {
	return &blockProcessor{
		ctx:                  ctx,
		core:                 core,
		headerCh:             newBlocks,
		blockRecorderFields:  make(map[string]interface{}, 11),
		acnPeerFields:        make(map[string]interface{}, 4),
		blockTimestampFields: make(map[string]interface{}, 1),
		qcAbsenteesFields:    make(map[string]interface{}, 2),
		apAbsenteesFields:    make(map[string]interface{}, 2),
		qcTags:               make(map[string]string, 1),
		apTags:               make(map[string]string, 1),
	}
}

func (bp *blockProcessor) Process() {
	go func() {
		for {
			select {
			case <-bp.ctx.Done():
				return
			case header, ok := <-bp.headerCh:
				if !ok {
					return
				}
				slog.Debug("new block received", "number", header.Number.Uint64())
				bp.core.blockCache.Add(header)
				bp.recordACNPeers(header)
				bp.core.epochInfoCache.Add(header)
				bp.core.markProcessed(header.Number.Uint64(), header.Number.Uint64())
				bp.recordBlockTimestamp(header)
				bp.recordBlock(header)
			}
		}
	}()
}

func (bp *blockProcessor) recordBlockTimestamp(header *types.Header) {
	bp.blockTimestampFields["block_timestamp"] = int64(header.Time)
	ts := time.Unix(int64(header.Number.Uint64()), 0)
	bp.core.dbHandler.WritePoint(BlockTimestamp, nil, bp.blockTimestampFields, ts)
}

func (bp *blockProcessor) recordACNPeers(header *types.Header) {

	now := time.Now()
	if now.Unix()-int64(header.Time) > 5 {
		slog.Debug("block older than 5 seconds. not retrieving acn peers")
		return
	}

	var result []*p2p.PeerInfo
	con := bp.core.cp.GetRPCConnection()
	err := con.Client.CallContext(bp.ctx, &result, "admin_acnPeers")
	if err != nil {
		slog.Error("Error fetching ACN peers", "error", err)
		return
	}

	tags := map[string]string{}
	ts := time.Unix(int64(header.Time), 0)
	for _, peer := range result {
		bp.acnPeerFields["enode"] = peer.Enode
		bp.acnPeerFields["block"] = header.Number.Uint64()
		bp.acnPeerFields["localAddress"] = peer.Network.LocalAddress
		bp.acnPeerFields["RemoteAddress"] = peer.Network.RemoteAddress
		tags["id"] = peer.ID

		bp.core.dbHandler.WritePoint(acnPeers, tags, bp.acnPeerFields, ts)
	}
	bp.core.dbHandler.Flush()
}

func (bp *blockProcessor) recordBlock(header *types.Header) {

	number := header.Number
	slog.Debug("Recording block", "num", number.Uint64())
	autCommittee := make([]autonity.AutonityCommitteeMember, 0)
	committeeMembers := make([]types.CommitteeMember, 0)
	epochInfo := bp.core.epochInfoCache.Get(number)
	if epochInfo == nil {
		slog.Error("not pushing the block in DB due to processing errors", "number", number)
		return
	}
	autCommittee = epochInfo.Committee
	committeeMembers = helper.AutCommitteeToCommittee(autCommittee)

	com := &types.Committee{
		Members: committeeMembers,
	}
	skippedProposers := make([]common.Address, 0)
	for i := 0; i < int(header.Round); i++ {
		sk := com.Proposer(header.Number.Uint64(), int64(header.Round))
		skippedProposers = append(skippedProposers, sk)
	}

	getMembers := func(ag *types.AggregateSignature) []common.Address {
		var members []common.Address
		if ag == nil {
			return members
		}
		err := ag.Signers.Validate(len(autCommittee))
		if err != nil {
			return members
		}
		indexes := ag.Signers.Flatten()
		for _, index := range indexes {
			member := autCommittee[index]
			members = append(members, member.Addr)
		}
		return members
	}

	absentees := func(signers []common.Address) []common.Address {
		ab := make([]common.Address, 0)
		for _, cm := range autCommittee {
			found := false
			for _, signer := range signers {
				if cm.Addr == signer {
					found = true
					break
				}
			}
			if !found {
				ab = append(ab, cm.Addr)
			}
		}
		return ab
	}
	qcAbsentees := absentees(getMembers(header.QuorumCertificate))
	apAbsentees := absentees(getMembers(header.ActivityProof))

	cm := make([]common.Address, 0, len(autCommittee))
	for _, mem := range autCommittee {
		cm = append(cm, mem.Addr)
	}
	bp.blockRecorderFields["qc-absentees"] = qcAbsentees
	bp.blockRecorderFields["ap-absentees"] = apAbsentees
	bp.blockRecorderFields["epoch"] = epochInfo.EpochBlock.Uint64()
	bp.blockRecorderFields["skipped-proposer"] = skippedProposers
	bp.blockRecorderFields["num-qc-absentees"] = len(qcAbsentees)
	bp.blockRecorderFields["num-ap-absentees"] = len(apAbsentees)
	bp.blockRecorderFields["round"] = header.Round
	bp.blockRecorderFields["activityProofRound"] = header.ActivityProofRound
	bp.blockRecorderFields["proposer"] = header.Coinbase
	bp.blockRecorderFields["committee"] = autCommittee
	bp.blockRecorderFields["block"] = number.Uint64()

	ts := time.Unix(int64(header.Time), 0)

	bp.core.dbHandler.WritePoint(BlockHeader, nil, bp.blockRecorderFields, ts)
	bp.trackInactivity(header, qcAbsentees, apAbsentees)
}

func (bp *blockProcessor) trackInactivity(header *types.Header, qcAbsentees, apAbsentees []common.Address) {

	ts := time.Unix(int64(header.Time), 0)
	bp.qcAbsenteesFields["block"] = header.Number.Uint64()
	bp.qcAbsenteesFields["absent"] = 1
	for _, m := range qcAbsentees {
		bp.qcTags["validator"] = m.String()
		bp.core.dbHandler.WritePoint(QCAbsentees, bp.qcTags, bp.qcAbsenteesFields, ts)
	}
	bp.apAbsenteesFields["block"] = header.Number.Uint64()
	bp.apAbsenteesFields["absent"] = 1
	for _, m := range apAbsentees {
		bp.apTags["validator"] = m.String()
		bp.core.dbHandler.WritePoint(APAbsentees, bp.apTags, bp.apAbsenteesFields, ts)
	}
}
