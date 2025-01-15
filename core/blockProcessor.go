package core

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/p2p"

	"Seer/interfaces"
)

var (
	acnPeers    = "ACNPeers"
	BlockHeader = "BlockHeader"
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
	for {
		select {
		case <-bp.ctx.Done():
			return
		case block, ok := <-bp.core.newBlocks:
			if !ok {
				return
			}
			//todo concurrency
			slog.Info("new block received", "number", block.Number().Uint64())
			bp.core.markProcessed(block.NumberU64(), block.NumberU64())
			bp.recordBlock(block)
			bp.recordACNPeers(block)

			bp.core.blockCache.Add(block)
			bp.core.epochInfoCache.Add(block)
		}
	}
}

func (bp *blockProcessor) recordACNPeers(block *types.Block) {
	now := time.Now().Unix()
	if now-int64(block.Time()) > 5 {
		slog.Debug("block older than 5 seconds. not retrieving acn peers")
		return
	}

	var result []*p2p.PeerInfo
	con := bp.core.cp.GetRPCConnection()
	defer bp.core.cp.PutRPCConnection(con)
	err := con.Client.CallContext(bp.ctx, &result, "admin_acnPeers")
	if err != nil {
		slog.Error("Error fetching ACN peers", "error", err)
		return
	}

	tags := map[string]string{}
	ts := time.Unix(int64(block.NumberU64()), 0)
	for _, peer := range result {
		fields := map[string]interface{}{}
		fields["enode"] = peer.Enode
		fields["localAddress"] = peer.Network.LocalAddress
		fields["RemoteAddress"] = peer.Network.RemoteAddress
		tags["id"] = peer.ID

		bp.core.dbHandler.WritePoint(acnPeers, tags, fields, ts)
	}
	bp.core.dbHandler.Flush()
}

func (bp *blockProcessor) recordBlock(block *types.Block) {
	tags := map[string]string{}

	header := block.Header()
	fields := map[string]interface{}{}
	fields["round"] = header.Round
	fields["activityProofRound"] = header.ActivityProofRound
	fields["proposer"] = header.Coinbase

	slog.Debug("Recording block", "num", block.NumberU64())

	//TODO: review correct epoch is pulled
	autCommittee := make([]autonity.AutonityCommitteeMember, 0)
	if block.IsEpochHead() {
		fields["epoch"] = block.NumberU64()
		committee := header.Epoch.Committee
		for _, member := range committee.Members {
			autCommittee = append(autCommittee, autonity.AutonityCommitteeMember{
				Addr:         member.Address,
				ConsensusKey: member.ConsensusKeyBytes,
				VotingPower:  member.VotingPower,
			})
		}
	} else {
		epochInfo := bp.core.epochInfoCache.Get(block.Number())
		if epochInfo == nil {
			slog.Error("not pushing the block in DB due to processing errors", "number", block.NumberU64())
			return
		}
		fields["epoch"] = epochInfo.EpochBlock.Uint64()
		autCommittee = epochInfo.Committee
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

	fields["qc-absentees"] = strings.Join(absentees(getMembers(header.QuorumCertificate)), ",")
	fields["ap-absentees"] = strings.Join(absentees(getMembers(header.ActivityProof)), ",")

	cm := make([]string, 0, len(autCommittee))
	for _, mem := range autCommittee {
		cm = append(cm, mem.Addr.String())
	}
	fields["committee"] = strings.Join(cm, ",")

	ts := time.Unix(int64(block.NumberU64()), 0)
	bp.core.dbHandler.WritePoint(BlockHeader, tags, fields, ts)
}
