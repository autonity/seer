package core

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/p2p"

	"seer/helper"
	"seer/interfaces"
)

var (
	acnPeers        = "ACNPeers"
	BlockHeader     = "BlockHeader"
	InactivityScore = "InactivityScore"
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
				bp.recordBlock(block)
				bp.recordACNPeers(block)

				bp.core.blockCache.Add(block)
				//bp.core.epochInfoCache.Add(block)
			}
		}
	}()
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
	committeeMembers := make([]types.CommitteeMember, 0)
	if block.IsEpochHead() {
		fields["epoch"] = block.NumberU64()
		committeeMembers = header.Epoch.Committee.Members
		autCommittee = helper.CommitteeToAutCommittee(committeeMembers)

	} else {
		epochInfo := bp.core.epochInfoCache.Get(block.Number())
		if epochInfo == nil {
			slog.Error("not pushing the block in DB due to processing errors", "number", block.NumberU64())
			return
		}
		fields["epoch"] = epochInfo.EpochBlock.Uint64()
		autCommittee = epochInfo.Committee
		committeeMembers = helper.AutCommitteeToCommittee(autCommittee)
	}

	com := &types.Committee{
		Members: committeeMembers,
	}
	skippedProposers := make([]string, 0)
	for i := 0; i < int(header.Round); i++ {
		sk := com.Proposer(header.Number.Uint64(), int64(header.Round))
		skippedProposers = append(skippedProposers, sk.String())
	}
	fields["skipped-proposer"] = strings.Join(skippedProposers, ",")

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
	fields["qc-absentees"] = strings.Join(qcAbsentees, ",")
	fields["ap-absentees"] = strings.Join(apAbsentees, ",")

	fields["num-qc-absentees"] = len(qcAbsentees)
	fields["num-ap-absentees"] = len(apAbsentees)

	cm := make([]string, 0, len(autCommittee))
	for _, mem := range autCommittee {
		cm = append(cm, mem.Addr.String())
	}
	fields["committee"] = strings.Join(cm, ",")

	ts := time.Unix(int64(block.NumberU64()), 0)
	bp.core.dbHandler.WritePoint(BlockHeader, tags, fields, ts)
	go bp.trackInactivity(block, com.Members)
}

func (bp *blockProcessor) trackInactivity(block *types.Block, committee []types.CommitteeMember) {
	con := bp.core.cp.GetWebSocketConnection()
	omissionBindings, err := autonity.NewOmissionAccountability(helper.OmissionAccountabilityContractAddress, con.Client)
	if err != nil {
		slog.Error("unable to create autonity bindings", "error", err)
	}

	fields := make(map[string]interface{}, 0)
	tags := make(map[string]string, 0)
	ts := time.Unix(int64(block.NumberU64()), 0)
	for _, m := range committee {
		inactivityScore, err := omissionBindings.GetInactivityScore(&bind.CallOpts{
			BlockNumber: block.Number(),
		}, m.Address)
		if err != nil || inactivityScore == nil {
			continue
		}
		fields["score"] = inactivityScore.Uint64()
		tags["validator"] = m.Address.String()
		bp.core.dbHandler.WritePoint(InactivityScore, tags, fields, ts)
	}
}
