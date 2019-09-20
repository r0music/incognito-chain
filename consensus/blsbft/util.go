package blsbft

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/incognitochain/incognito-chain/consensus"
	"github.com/incognitochain/incognito-chain/consensus/signatureschemes/blsmultisig"
	"github.com/incognitochain/incognito-chain/incognitokey"

	"github.com/incognitochain/incognito-chain/common"
)

func (e *BLSBFT) getTimeSinceLastBlock() time.Duration {
	return time.Since(time.Unix(int64(e.Chain.GetLastBlockTimeStamp()), 0))
}

func (e *BLSBFT) waitForNextRound() {
	timeSinceLastBlk := e.getTimeSinceLastBlock()
	if timeSinceLastBlk > e.Chain.GetMinBlkInterval() {
		return
	}
	//TODO: chunk time sleep into small time chunk -> if change view during sleep => break it
	fmt.Println("\n\nSleep for", e.Chain.GetMinBlkInterval()-timeSinceLastBlk, "\n\n")

	time.Sleep(e.Chain.GetMinBlkInterval() - timeSinceLastBlk)
}

func (e *BLSBFT) setState(state string) {
	e.RoundData.State = state
}

func (e *BLSBFT) getCurrentRound() int {
	round := int((e.getTimeSinceLastBlock().Seconds() - float64(e.Chain.GetMinBlkInterval().Seconds())) / timeout.Seconds())
	if round < 0 {
		return 1
	}

	return round + 1
}

func (e *BLSBFT) isInTimeFrame() bool {
	if e.Chain.CurrentHeight()+1 != e.RoundData.NextHeight {
		return false
	}

	if e.getCurrentRound() != e.RoundData.Round {
		return false
	}

	return true
}

func (e *BLSBFT) isHasMajorityVotes() bool {
	// e.RoundData.lockVotes.Lock()
	// defer e.RoundData.lockVotes.Unlock()
	e.lockEarlyVotes.Lock()
	defer e.lockEarlyVotes.Unlock()
	roundKey := getRoundKey(e.RoundData.NextHeight, e.RoundData.Round)
	earlyVote, ok := e.EarlyVotes[roundKey]
	committeeSize := len(e.RoundData.Committee)
	if ok {
		wg := sync.WaitGroup{}
		blockHashBytes := e.RoundData.BlockHash.GetBytes()
		for k, v := range earlyVote {
			wg.Add(1)
			go func(validatorKey string, voteData vote) {
				defer wg.Done()
				validatorIdx := common.IndexOfStr(validatorKey, e.RoundData.CommitteeBLS.StringList)
				if err := e.preValidateVote(blockHashBytes, &voteData, e.RoundData.Committee[validatorIdx].MiningPubKey[common.BridgeConsensus]); err == nil {
					// if err := validateSingleBLSSig(e.RoundData.Block.Hash(), vote.BLS, validatorIdx, e.RoundData.CommitteeBLS.ByteList); err != nil {
					// 	e.logger.Error(err)
					// 	continue
					// }
					e.RoundData.lockVotes.Lock()
					e.RoundData.Votes[validatorKey] = voteData
					e.RoundData.lockVotes.Unlock()
				} else {
					e.logger.Error(err)
				}
			}(k, v)
		}
		wg.Wait()
		if len(e.RoundData.Votes) > 2*committeeSize/3 {
			delete(e.EarlyVotes, roundKey)
		}
	}
	if len(e.RoundData.Votes) > 2*committeeSize/3 {
		return true
	}
	return false
}

func getRoundKey(nextHeight uint64, round int) string {
	return fmt.Sprint(nextHeight, "_", round)
}

func (e *BLSBFT) ExtractBridgeValidationData(block common.BlockInterface) ([][]byte, []int, error) {
	valData, err := DecodeValidationData(block.GetValidationField())
	if err != nil {
		return nil, nil, consensus.NewConsensusError(consensus.UnExpectedError, err)
	}
	return valData.BridgeSig, valData.ValidatiorsIdx, nil
}

func (e *BLSBFT) UpdateCommitteeBLSList() {
	committee := e.Chain.GetCommittee()
	if !reflect.DeepEqual(e.RoundData.Committee, committee) {
		e.RoundData.Committee = committee
		e.RoundData.CommitteeBLS.ByteList = []blsmultisig.PublicKey{}
		e.RoundData.CommitteeBLS.StringList = []string{}
		for _, member := range e.RoundData.Committee {
			e.RoundData.CommitteeBLS.ByteList = append(e.RoundData.CommitteeBLS.ByteList, member.MiningPubKey[consensusName])
		}
		committeeBLSString, err := incognitokey.ExtractPublickeysFromCommitteeKeyList(e.RoundData.Committee, consensusName)
		if err != nil {
			e.logger.Error(err)
			return
		}
		e.RoundData.CommitteeBLS.StringList = committeeBLSString
	}
}

func (e *BLSBFT) InitRoundData() {
	roundKey := getRoundKey(e.RoundData.NextHeight, e.RoundData.Round)
	if _, ok := e.Blocks[roundKey]; ok {
		delete(e.Blocks, roundKey)
	}
	e.RoundData.NextHeight = e.Chain.CurrentHeight() + 1
	e.RoundData.Round = e.getCurrentRound()
	e.RoundData.Votes = make(map[string]vote)
	e.RoundData.Block = nil
	e.RoundData.BlockHash = nil
	e.RoundData.NotYetSendVote = true
	e.RoundData.LastProposerIndex = e.Chain.GetLastProposerIndex()
	e.UpdateCommitteeBLSList()
}
