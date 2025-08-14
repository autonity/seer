package core

import (
	"context"
	"log/slog"
	"math/big"
	"sync"
	"time"

	"github.com/autonity/autonity/accounts/abi"
	"github.com/autonity/autonity/accounts/abi/bind"
	"github.com/autonity/autonity/autonity/bindings"
	"github.com/autonity/autonity/common"
	"github.com/autonity/autonity/core/types"
	"github.com/autonity/autonity/p2p"
	"github.com/autonity/autonity/params/generated"

	"seer/helper"
	"seer/interfaces"
)

var (
	acnPeers          = "ACNPeers"
	BlockHeader       = "BlockHeader"
	BlockTimestamp    = "BlockTimestamp"
	QCAbsentees       = "QuorumCertificateAbsentees"
	APAbsentees       = "ActivityProofAbsentees"
	BlockTransactions = "BlockTransactions"
)

type SeerBlock struct {
	autonityBlock    *types.Block
	transactionCount int
}

type blockProcessor struct {
	core                 *core
	ctx                  context.Context
	blockCh              chan *SeerBlock
	blockRecorderFields  map[string]interface{}
	blockTimestampFields map[string]interface{}
	qcAbsenteesFields    map[string]interface{}
	apAbsenteesFields    map[string]interface{}
	isLive               bool // differenciates whether we are processing blocks history or live
	signer               types.Signer
}

func NewBlockProcessor(ctx context.Context, core *core, newBlocks chan *SeerBlock, isLive bool, chainID *big.Int) interfaces.Processor {
	bp := &blockProcessor{
		ctx:                  ctx,
		core:                 core,
		blockCh:              newBlocks,
		isLive:               isLive,
		blockRecorderFields:  make(map[string]interface{}, 11),
		blockTimestampFields: make(map[string]interface{}, 1),
		qcAbsenteesFields:    make(map[string]interface{}, 2),
		apAbsenteesFields:    make(map[string]interface{}, 2),
	}
	bp.signer = types.NewLondonSigner(chainID)
	return bp
}

func (bp *blockProcessor) Process() {
	go func() {
		for {
			select {
			case <-bp.ctx.Done():
				return
			case seerBlock, ok := <-bp.blockCh:
				if !ok {
					return
				}
				block := seerBlock.autonityBlock
				slog.Debug("new block received", "number", block.NumberU64())
				header := block.Header()
				bp.core.blockCache.Add(block)
				bp.core.epochInfoCache.Add(header)

				bp.recordACNPeers(header)
				bp.core.markProcessedBlock(header.Number.Uint64(), header.Number.Uint64())
				bp.recordBlockTimestamp(header)
				bp.recordBlock(header)
				bp.checkOracleVote(block)
				bp.recordTxCount(block, seerBlock.transactionCount)
			}
		}
	}()
}

func (bp *blockProcessor) recordBlockTimestamp(header *types.Header) {
	bp.blockTimestampFields["block_timestamp"] = int64(header.Time)
	ts := time.Unix(int64(header.Number.Uint64()), 0)
	bp.core.dbHandler.WritePoint(BlockTimestamp, nil, bp.blockTimestampFields, ts)
}

func (bp *blockProcessor) recordTxCount(block *types.Block, txCount int) {
	ts := time.Unix(int64(block.Header().Time), 0)
	fields := make(map[string]interface{})
	fields["transactionCount"] = txCount
	fields["block"] = block.NumberU64()
	bp.core.dbHandler.WritePoint(BlockTransactions, nil, fields, ts)
}

func (bp *blockProcessor) recordACNPeers(header *types.Header) {

	now := time.Now()
	if now.Unix()-int64(header.Time) > 5 {
		return
	}

	var result []*p2p.PeerInfo
	con := bp.core.cp.GetRPCConnection()

	err := con.Client.CallContext(bp.ctx, &result, "aut_acnPeers")
	if err != nil {
		slog.Error("Error fetching ACN peers", "error", err)
		return
	}

	tags := map[string]string{}
	ts := time.Unix(int64(header.Time), 0)
	for _, peer := range result {
		acnPeerFields := make(map[string]interface{}, 4)
		acnPeerFields["enode"] = peer.Enode
		acnPeerFields["block"] = header.Number.Uint64()
		acnPeerFields["localAddress"] = peer.Network.LocalAddress
		acnPeerFields["RemoteAddress"] = peer.Network.RemoteAddress
		tags["id"] = peer.ID

		bp.core.dbHandler.WritePoint(acnPeers, tags, acnPeerFields, ts)
	}
	bp.core.dbHandler.Flush()
}

func (bp *blockProcessor) recordBlock(header *types.Header) {
	clear(bp.blockRecorderFields)
	clear(bp.qcAbsenteesFields)
	clear(bp.apAbsenteesFields)

	number := header.Number
	slog.Debug("Recording block", "num", number.Uint64())
	autCommittee := make([]bindings.IAutonityCommitteeMember, 0)
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
		sk := com.Proposer(header.Number.Uint64(), int64(i))
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
		ag.Signers.ForEachDistinctSigner(func(signerIndex int) {
			member := autCommittee[signerIndex]
			members = append(members, member.Addr)
		})
		return members
	}

	absentees := func(signers []common.Address) []common.Address {
		signerSet := make(map[common.Address]struct{}, len(signers))
		for _, signer := range signers {
			signerSet[signer] = struct{}{}
		}

		ab := make([]common.Address, 0)
		for _, cm := range autCommittee {
			if _, found := signerSet[cm.Addr]; !found {
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
	bp.blockRecorderFields["committee"] = cm
	bp.blockRecorderFields["block"] = number.Uint64()

	ts := time.Unix(int64(header.Time), 0)

	bp.core.dbHandler.WritePoint(BlockHeader, nil, bp.blockRecorderFields, ts)
	bp.trackInactivity(header, qcAbsentees, apAbsentees)
}

func (bp *blockProcessor) trackInactivity(header *types.Header, qcAbsentees, apAbsentees []common.Address) {

	ts := time.Unix(int64(header.Time), 0)
	bp.qcAbsenteesFields["block"] = header.Number.Uint64()
	bp.qcAbsenteesFields["absent"] = 1
	qcTags := make(map[string]string)
	for _, m := range qcAbsentees {
		qcTags["validator"] = m.String()
		bp.core.dbHandler.WritePoint(QCAbsentees, qcTags, bp.qcAbsenteesFields, ts)
	}
	bp.apAbsenteesFields["block"] = header.Number.Uint64()
	bp.apAbsenteesFields["absent"] = 1
	apTags := make(map[string]string)
	for _, m := range apAbsentees {
		apTags["validator"] = m.String()
		bp.core.dbHandler.WritePoint(APAbsentees, apTags, bp.apAbsenteesFields, ts)
	}
}

func (bp *blockProcessor) checkOracleVote(block *types.Block) {
	for _, tx := range block.Transactions() {
		if tx.To() != nil && *tx.To() == helper.OracleContractAddress {
			data := tx.Data()
			if len(data) < 4 {
				continue
			}
			voteMethod, err := generated.OracleAbi.MethodById(tx.Data())
			if err != nil {
				slog.Error("Error fetching method by ID", "error", err, "block", block.NumberU64())
				continue
			}
			t := tx
			if voteMethod.Name == "vote" {
				bp.processVoteTransaction(t, voteMethod, block.Number(), block.Time())
			}
		}
	}
}

func (bp *blockProcessor) processVoteTransaction(tx *types.Transaction, voteMethod *abi.Method, blockNumber *big.Int, blockTime uint64) {
	var receipt *types.Receipt
	var symbols []string
	var round *big.Int

	wg := sync.WaitGroup{}
	wg.Add(1)
	con := bp.core.cp.GetWebSocketConnection()
	go func() {
		defer wg.Done()
		var err error
		receipt, err = con.Client.TransactionReceipt(bp.ctx, tx.Hash())
		if err != nil {
			slog.Error("Error fetching vote transaction receipt", "error", err)
			return
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		var err error
		oracleBindings, err := bindings.NewOracle(helper.OracleContractAddress, con.Client)
		if err != nil {
			slog.Error("unable to create oracle bindings", "error", err)
			return
		}
		symbols, err = oracleBindings.GetSymbols(
			&bind.CallOpts{
				BlockNumber: blockNumber,
			})
		if err != nil {
			slog.Error("unable to get oracle symbols ", "error", err)
			return
		}
		round, err = oracleBindings.GetRound(&bind.CallOpts{
			BlockNumber: blockNumber,
		})
	}()

	fields := make(map[string]interface{}, 5)
	tags := make(map[string]string, 2)
	ts := time.Unix(int64(blockTime), 0)

	sender, err := bp.signer.Sender(tx)
	if err != nil {
		slog.Error("unable to get tx sender info", "error", err)
		return
	}
	tags["voter"] = sender.Hex()
	fields["voted"] = true
	fields["block"] = blockNumber.Uint64()

	voteData, err := voteMethod.Inputs.Unpack(tx.Data()[4:])
	if err != nil {
		slog.Error("unable to unpack vote method ", "error", err)
		return
	}
	reports := voteData[1].([]struct {
		Price      *big.Int `json:"price"`
		Confidence uint8    `json:"confidence"`
	})
	wg.Wait()

	if receipt == nil || len(symbols) == 0 || round == nil {
		slog.Error("unable to record vote transaction.. retry")
		return
	}

	fields["round"] = round.Uint64()
	if receipt.Status == 0 {
		fields["status"] = "failed"
	} else {
		fields["status"] = "success"
	}

	if len(symbols) != len(reports) {
		tags["symbol"] = "symbol_len_mismatch"
		bp.core.dbHandler.WritePoint("OracleVote", tags, fields, ts)
		return
	}

	for i, report := range reports {
		tags["symbol"] = symbols[i]
		fields["reported-price"] = report.Price
		fields["confidence"] = report.Confidence
		bp.core.dbHandler.WritePoint("OracleVote", tags, fields, ts)
	}
}
