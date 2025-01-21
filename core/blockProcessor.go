package core

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/autonity/autonity/autonity"
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
	core *core
	ctx  context.Context
}

func NewBlockProcessor(ctx context.Context, core *core) interfaces.Processor {
	return &blockProcessor{
		core: core,
		ctx:  ctx,
	}
}

func (bp *blockProcessor) Process() {
	go func() {
		for {
			select {
			case <-bp.ctx.Done():
				return
			case block, ok := <-bp.core.newBlocks:
				if !ok {
					return
				}
				slog.Debug("new block received", "number", block.Number().Uint64())
				bp.core.markProcessed(block.NumberU64(), block.NumberU64())
				bp.recordBlockTimestamp(block)
				bp.recordBlock(block)
				bp.recordACNPeers(block)

				bp.core.blockCache.Add(block)
				bp.core.epochInfoCache.Add(block)
			}
		}
	}()
}

func (bp *blockProcessor) recordBlockTimestamp(block *types.Block) {
	fields := map[string]interface{}{}
	fields["block_timestamp"] = int64(block.Time())
	ts := time.Unix(int64(block.NumberU64()), 0)
	bp.core.dbHandler.WritePoint(BlockTimestamp, nil, fields, ts)
}

func (bp *blockProcessor) recordACNPeers(block *types.Block) {
	now := time.Now().Unix()
	if now-int64(block.Time()) > 5 {
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
	//ts := time.Unix(int64(block.NumberU64()), 0)
	ts := time.Unix(int64(block.Time()), 0)
	for _, peer := range result {
		fields := map[string]interface{}{}
		fields["enode"] = peer.Enode
		fields["block"] = block.NumberU64()
		fields["localAddress"] = peer.Network.LocalAddress
		fields["RemoteAddress"] = peer.Network.RemoteAddress
		tags["id"] = peer.ID

		bp.core.dbHandler.WritePoint(acnPeers, tags, fields, ts)
	}
	bp.core.dbHandler.Flush()
}

func (bp *blockProcessor) recordBlock(block *types.Block) {

	header := block.Header()

	slog.Debug("Recording block", "num", block.NumberU64())
	autCommittee := make([]autonity.AutonityCommitteeMember, 0)
	committeeMembers := make([]types.CommitteeMember, 0)
	epochInfo := bp.core.epochInfoCache.Get(block.Number())
	if epochInfo == nil {
		slog.Error("not pushing the block in DB due to processing errors", "number", block.NumberU64())
		return
	}
	autCommittee = epochInfo.Committee
	committeeMembers = helper.AutCommitteeToCommittee(autCommittee)

	com := &types.Committee{
		Members: committeeMembers,
	}
	skippedProposers := make([]string, 0)
	for i := 0; i < int(header.Round); i++ {
		sk := com.Proposer(header.Number.Uint64(), int64(header.Round))
		skippedProposers = append(skippedProposers, sk.String())
	}

	getMembers := func(ag *types.AggregateSignature) []string {
		var members []string
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
			members = append(members, member.Addr.String())
		}
		return members
	}

	absentees := func(signers []string) []string {
		ab := make([]string, 0)
		for _, cm := range autCommittee {
			found := false
			for _, signer := range signers {
				if cm.Addr.String() == signer {
					found = true
					break
				}
			}
			if !found {
				ab = append(ab, cm.Addr.String())
			}
		}
		return ab
	}
	qcAbsentees := absentees(getMembers(header.QuorumCertificate))
	apAbsentees := absentees(getMembers(header.ActivityProof))

	cm := make([]string, 0, len(autCommittee))
	for _, mem := range autCommittee {
		cm = append(cm, mem.Addr.String())
	}
	fields := map[string]interface{}{}
	fields["qc-absentees"] = strings.Join(qcAbsentees, ",")
	fields["ap-absentees"] = strings.Join(apAbsentees, ",")
	fields["epoch"] = epochInfo.EpochBlock.Uint64()
	fields["skipped-proposer"] = strings.Join(skippedProposers, ",")
	fields["num-qc-absentees"] = len(qcAbsentees)
	fields["num-ap-absentees"] = len(apAbsentees)
	fields["round"] = header.Round
	fields["activityProofRound"] = header.ActivityProofRound
	fields["proposer"] = header.Coinbase
	fields["committee"] = strings.Join(cm, ",")
	fields["block"] = block.NumberU64()

	ts := time.Unix(int64(block.Time()), 0)

	bp.core.dbHandler.WritePoint(BlockHeader, nil, fields, ts)
	go bp.trackInactivity(block, com.Members, qcAbsentees, apAbsentees)
}

func (bp *blockProcessor) trackInactivity(block *types.Block, committee []types.CommitteeMember, qcAbsentees, apAbsentees []string) {

	fields := make(map[string]interface{}, 0)
	tags := make(map[string]string, 0)
	ts := time.Unix(int64(block.Time()), 0)
	fields["block"] = block.NumberU64()
	for _, m := range qcAbsentees {
		tags["validator"] = m
		fields["absent"] = 1
		bp.core.dbHandler.WritePoint(QCAbsentees, tags, fields, ts)
	}
	for _, m := range apAbsentees {
		tags["validator"] = m
		fields["absent"] = 1
		bp.core.dbHandler.WritePoint(APAbsentees, tags, fields, ts)
	}
}
