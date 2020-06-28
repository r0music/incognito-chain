package committeestate

import (
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/instruction"
	"reflect"
)

type BeaconCommitteeStateEnvironment struct {
	beaconHeight                               uint64
	beaconHash                                 common.Hash
	paramEpoch                                 uint64
	beaconInstructions                         [][]string
	newBeaconHeight                            uint64
	epochLength                                uint64
	epochBreakPointSwapNewKey                  []uint64
	randomNumber                               int64
	randomFlag                                 bool
	isRandomTime                               bool
	assignOffset                               int
	activeShard                                int
	allCandidateSubstituteCommittee            []string
	selectBeaconNodeSerializedPubkeyV2         map[uint64][]string
	selectBeaconNodeSerializedPaymentAddressV2 map[uint64][]string
	preSelectBeaconNodeSerializedPubkey        []string
	selectShardNodeSerializedPubkeyV2          map[uint64][]string
	selectShardNodeSerializedPaymentAddressV2  map[uint64][]string
	preSelectShardNodeSerializedPubkey         []string
}

func NewBeaconCommitteeStateEnvironment(beaconInstructions [][]string, newBeaconHeight uint64, epochLength uint64, epochBreakPointSwapNewKey []uint64, randomNumber int64, randomFlag bool) *BeaconCommitteeStateEnvironment {
	return &BeaconCommitteeStateEnvironment{beaconInstructions: beaconInstructions, newBeaconHeight: newBeaconHeight, epochLength: epochLength, epochBreakPointSwapNewKey: epochBreakPointSwapNewKey, randomNumber: randomNumber, randomFlag: randomFlag}
}

type BeaconCommitteeEngine struct {
	beaconHeight                      uint64
	beaconHash                        common.Hash
	beaconCommitteeStateV1            *BeaconCommitteeStateV1
	uncommittedBeaconCommitteeStateV1 *BeaconCommitteeStateV1
}

func NewBeaconCommitteeEngine(beaconHeight uint64, beaconHash common.Hash, beaconCommitteeStateV1 *BeaconCommitteeStateV1) *BeaconCommitteeEngine {
	return &BeaconCommitteeEngine{
		beaconHeight:                      beaconHeight,
		beaconHash:                        beaconHash,
		beaconCommitteeStateV1:            beaconCommitteeStateV1,
		uncommittedBeaconCommitteeStateV1: NewBeaconCommitteeStateV1(),
	}
}

type BeaconCommitteeStateV1 struct {
	beaconCommittee             []incognitokey.CommitteePublicKey
	beaconSubstitute            []incognitokey.CommitteePublicKey
	currentEpochShardCandidate  []incognitokey.CommitteePublicKey
	currentEpochBeaconCandidate []incognitokey.CommitteePublicKey
	nextEpochShardCandidate     []incognitokey.CommitteePublicKey
	nextEpochBeaconCandidate    []incognitokey.CommitteePublicKey
	shardCommittee              map[byte][]incognitokey.CommitteePublicKey
	shardSubstitute             map[byte][]incognitokey.CommitteePublicKey
	autoStaking                 map[string]bool
	rewardReceiver              map[string]string
}

func NewBeaconCommitteeStateV1() *BeaconCommitteeStateV1 {
	return &BeaconCommitteeStateV1{
		shardCommittee:  make(map[byte][]incognitokey.CommitteePublicKey),
		shardSubstitute: make(map[byte][]incognitokey.CommitteePublicKey),
		autoStaking:     make(map[string]bool),
		rewardReceiver:  make(map[string]string),
	}
}

func NewBeaconCommitteeStateV1WithValue(beaconCommittee []incognitokey.CommitteePublicKey, beaconSubstitute []incognitokey.CommitteePublicKey, candidateShardWaitingForCurrentRandom []incognitokey.CommitteePublicKey, candidateBeaconWaitingForCurrentRandom []incognitokey.CommitteePublicKey, candidateShardWaitingForNextRandom []incognitokey.CommitteePublicKey, candidateBeaconWaitingForNextRandom []incognitokey.CommitteePublicKey, shardCommittee map[byte][]incognitokey.CommitteePublicKey, shardSubstitute map[byte][]incognitokey.CommitteePublicKey, autoStaking map[string]bool, rewardReceiver map[string]string) *BeaconCommitteeStateV1 {
	return &BeaconCommitteeStateV1{beaconCommittee: beaconCommittee, beaconSubstitute: beaconSubstitute, currentEpochShardCandidate: candidateShardWaitingForCurrentRandom, currentEpochBeaconCandidate: candidateBeaconWaitingForCurrentRandom, nextEpochShardCandidate: candidateShardWaitingForNextRandom, nextEpochBeaconCandidate: candidateBeaconWaitingForNextRandom, shardCommittee: shardCommittee, shardSubstitute: shardSubstitute, autoStaking: autoStaking, rewardReceiver: rewardReceiver}
}

func (engine *BeaconCommitteeEngine) Commit() error {
	if reflect.DeepEqual(engine.uncommittedBeaconCommitteeStateV1, NewBeaconCommitteeStateV1()) {
		return NewCommitteeStateError(ErrEmptyUncommittedBeaconCommitteeState, fmt.Errorf("%+v", engine.uncommittedBeaconCommitteeStateV1))
	}
	engine.beaconCommitteeStateV1 = engine.uncommittedBeaconCommitteeStateV1
	engine.uncommittedBeaconCommitteeStateV1 = NewBeaconCommitteeStateV1()
	return nil
}

func (engine *BeaconCommitteeEngine) Abort() {
	engine.uncommittedBeaconCommitteeStateV1 = NewBeaconCommitteeStateV1()
}

func (engine BeaconCommitteeEngine) GenerateUncommittedCommitteeRootHashes() ([]common.Hash, error) {
	hashes := []common.Hash{}
	if reflect.DeepEqual(engine.uncommittedBeaconCommitteeStateV1, NewBeaconCommitteeStateV1()) {
		return hashes, NewCommitteeStateError(ErrEmptyUncommittedBeaconCommitteeState, fmt.Errorf("%+v", engine.uncommittedBeaconCommitteeStateV1))
	}
	panic("implement me")
}

func (engine *BeaconCommitteeEngine) UpdateCommitteeState(env *BeaconCommitteeStateEnvironment) (*incognitokey.CommitteeChange, error) {
	b := engine.beaconCommitteeStateV1
	newB := b.cloneBeaconCommitteeStateV1()
	committeeChange := incognitokey.NewCommitteeChange()
	newBeaconCandidates := []incognitokey.CommitteePublicKey{}
	newShardCandidates := []incognitokey.CommitteePublicKey{}
	for _, inst := range env.beaconInstructions {
		if len(inst) == 0 {
			continue
		}
		tempNewBeaconCandidates, tempNewShardCandidates := []incognitokey.CommitteePublicKey{}, []incognitokey.CommitteePublicKey{}
		switch inst[0] {
		case instruction.STAKE_ACTION:
			stakeInstruction, err := instruction.ValidateAndImportStakeInstructionFromString(inst)
			if err != nil {
				Logger.log.Errorf("SKIP stake instruction %+v, error %+v", inst, err)
				continue
			}
			tempNewBeaconCandidates, tempNewShardCandidates = newB.processStakeInstruction(stakeInstruction, env)
		case instruction.SWAP_ACTION:
			swapInstruction, err := instruction.ValidateAndImportSwapInstructionFromString(inst)
			if err != nil {
				Logger.log.Errorf("SKIP swap instruction %+v, error %+v", inst, err)
				continue
			}
			tempNewBeaconCandidates, tempNewShardCandidates, err = newB.processSwapInstruction(swapInstruction, env, committeeChange)
			if err != nil {
				return nil, err
			}
		case instruction.STOP_AUTO_STAKE_ACTION:
			stopAutoStakeInstruction, err := instruction.ValidateAndImportStopAutoStakeInstructionFromString(inst)
			if err != nil {
				Logger.log.Errorf("SKIP stop auto stake instruction %+v, error %+v", inst, err)
			}
			newB.processStopAutoStakeInstruction(stopAutoStakeInstruction, env, committeeChange)
		}
		if len(tempNewBeaconCandidates) > 0 {
			newBeaconCandidates = append(newBeaconCandidates, tempNewBeaconCandidates...)
		}
		if len(tempNewShardCandidates) > 0 {
			newShardCandidates = append(newShardCandidates, tempNewShardCandidates...)
		}
	}
	newB.nextEpochBeaconCandidate = append(newB.nextEpochBeaconCandidate, newBeaconCandidates...)
	committeeChange.NextEpochBeaconCandidateAdded = append(committeeChange.NextEpochBeaconCandidateAdded, newBeaconCandidates...)
	newB.nextEpochShardCandidate = append(newB.nextEpochShardCandidate, newShardCandidates...)
	committeeChange.NextEpochShardCandidateAdded = append(committeeChange.NextEpochShardCandidateAdded, newShardCandidates...)
	if env.isRandomTime {
		committeeChange.CurrentEpochShardCandidateAdded = newB.nextEpochShardCandidate
		newB.currentEpochShardCandidate = newB.nextEpochShardCandidate
		committeeChange.CurrentEpochBeaconCandidateAdded = newB.nextEpochBeaconCandidate
		newB.currentEpochBeaconCandidate = newB.nextEpochBeaconCandidate
		Logger.log.Debug("Beacon Process: CandidateShardWaitingForCurrentRandom: ", newB.currentEpochShardCandidate)
		Logger.log.Debug("Beacon Process: CandidateBeaconWaitingForCurrentRandom: ", newB.currentEpochBeaconCandidate)
		// reset candidate list
		committeeChange.NextEpochShardCandidateRemoved = newB.nextEpochShardCandidate
		newB.nextEpochShardCandidate = []incognitokey.CommitteePublicKey{}
		committeeChange.NextEpochBeaconCandidateRemoved = newB.nextEpochBeaconCandidate
		newB.nextEpochBeaconCandidate = []incognitokey.CommitteePublicKey{}
	}
	if env.randomFlag {
		numberOfShardSubstitutes := make(map[byte]int)
		for shardID, shardSubstitute := range newB.shardSubstitute {
			numberOfShardSubstitutes[shardID] = len(shardSubstitute)
		}
		shardCandidatesStr, err := incognitokey.CommitteeKeyListToString(newB.currentEpochShardCandidate)
		if err != nil {
			panic(err)
		}
		remainShardCandidatesStr, assignedCandidates := assignShardCandidate(shardCandidatesStr, numberOfShardSubstitutes, env.randomNumber, env.assignOffset, env.activeShard)
		remainShardCandidates, err := incognitokey.CommitteeBase58KeyListToStruct(remainShardCandidatesStr)
		if err != nil {
			panic(err)
		}
		committeeChange.NextEpochShardCandidateAdded = append(committeeChange.NextEpochShardCandidateAdded, remainShardCandidates...)
		// append remain candidate into shard waiting for next random list
		newB.nextEpochShardCandidate = append(newB.nextEpochShardCandidate, remainShardCandidates...)
		// assign candidate into shard pending validator list
		for shardID, candidateListStr := range assignedCandidates {
			candidateList, err := incognitokey.CommitteeBase58KeyListToStruct(candidateListStr)
			if err != nil {
				panic(err)
			}
			committeeChange.ShardSubstituteAdded[shardID] = candidateList
			newB.shardSubstitute[shardID] = append(newB.shardSubstitute[shardID], candidateList...)
		}
		committeeChange.CurrentEpochShardCandidateRemoved = newB.currentEpochShardCandidate
		// delete CandidateShardWaitingForCurrentRandom list
		newB.currentEpochShardCandidate = []incognitokey.CommitteePublicKey{}
		// shuffle CandidateBeaconWaitingForCurrentRandom with current random number
		newBeaconSubstitute, err := ShuffleCandidate(newB.currentEpochBeaconCandidate, env.randomNumber)
		if err != nil {
			return nil, err
		}
		committeeChange.CurrentEpochBeaconCandidateRemoved = newB.currentEpochBeaconCandidate
		newB.currentEpochBeaconCandidate = []incognitokey.CommitteePublicKey{}
		committeeChange.BeaconSubstituteAdded = newBeaconSubstitute
		newB.beaconSubstitute = append(newB.beaconSubstitute, newBeaconSubstitute...)
	}
	err := newB.processAutoStakingChange(committeeChange)
	if err != nil {
		return nil, err
	}
	engine.uncommittedBeaconCommitteeStateV1 = newB
	return nil, nil
}

func (b *BeaconCommitteeStateV1) processStakeInstruction(
	stakeInstruction *instruction.StakeInstruction,
	env *BeaconCommitteeStateEnvironment,
) ([]incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey) {
	newBeaconCandidates := []incognitokey.CommitteePublicKey{}
	newShardCandidates := []incognitokey.CommitteePublicKey{}
	for index, candidate := range stakeInstruction.PublicKeyStructs {
		b.rewardReceiver[candidate.GetIncKeyBase58()] = stakeInstruction.RewardReceivers[index]
		b.autoStaking[stakeInstruction.PublicKeys[index]] = stakeInstruction.AutoStakingFlag[index]
	}
	if stakeInstruction.Chain == instruction.BEACON_INST {
		newBeaconCandidates = append(newBeaconCandidates, stakeInstruction.PublicKeyStructs...)
	} else {
		newShardCandidates = append(newShardCandidates, stakeInstruction.PublicKeyStructs...)
	}
	return newBeaconCandidates, newShardCandidates
}

func (b *BeaconCommitteeStateV1) processStopAutoStakeInstruction(
	stopAutoStakeInstruction *instruction.StopAutoStakeInstruction,
	env *BeaconCommitteeStateEnvironment,
	committeeChange *incognitokey.CommitteeChange,
) {
	for _, committeePublicKey := range stopAutoStakeInstruction.PublicKeys {
		if common.IndexOfStr(committeePublicKey, env.allCandidateSubstituteCommittee) == -1 {
			// if not found then delete auto staking data for this public key if present
			if _, ok := b.autoStaking[committeePublicKey]; ok {
				delete(b.autoStaking, committeePublicKey)
			}
		} else {
			// if found in committee list then turn off auto staking
			if _, ok := b.autoStaking[committeePublicKey]; ok {
				b.autoStaking[committeePublicKey] = false
				committeeChange.StopAutoStake = append(committeeChange.StopAutoStake, committeePublicKey)
			}
		}
	}
}

func (b *BeaconCommitteeStateV1) processSwapInstruction(
	swapInstruction *instruction.SwapInstruction,
	env *BeaconCommitteeStateEnvironment,
	committeeChange *incognitokey.CommitteeChange,
) ([]incognitokey.CommitteePublicKey, []incognitokey.CommitteePublicKey, error) {
	newBeaconCandidates := []incognitokey.CommitteePublicKey{}
	newShardCandidates := []incognitokey.CommitteePublicKey{}
	if common.IndexOfUint64(env.beaconHeight/env.paramEpoch, env.epochBreakPointSwapNewKey) > -1 || swapInstruction.IsReplace {
		err := b.processReplaceInstruction(swapInstruction, committeeChange)
		if err != nil {
			return newBeaconCandidates, newShardCandidates, err
		}
	} else {
		Logger.log.Debug("Swap Instruction In Public Keys", swapInstruction.InPublicKeys)
		Logger.log.Debug("Swap Instruction Out Public Keys", swapInstruction.OutPublicKeys)
		if swapInstruction.ChainID != instruction.BEACON_CHAIN_ID {
			shardID := byte(swapInstruction.ChainID)
			// delete in public key out of sharding pending validator list
			if len(swapInstruction.InPublicKeys) > 0 {
				shardSubstituteStr, err := incognitokey.CommitteeKeyListToString(b.shardSubstitute[shardID])
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				tempShardSubstitute, err := RemoveValidator(shardSubstituteStr, swapInstruction.InPublicKeys)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// update shard pending validator
				committeeChange.ShardSubstituteRemoved[shardID] = append(committeeChange.ShardSubstituteRemoved[shardID], swapInstruction.InPublicKeyStructs...)
				b.shardSubstitute[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(tempShardSubstitute)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// add new public key to committees
				committeeChange.ShardCommitteeAdded[shardID] = append(committeeChange.ShardCommitteeAdded[shardID], swapInstruction.InPublicKeyStructs...)
				b.shardCommittee[shardID] = append(b.shardCommittee[shardID], swapInstruction.InPublicKeyStructs...)
			}
			// delete out public key out of current committees
			if len(swapInstruction.OutPublicKeys) > 0 {
				//for _, value := range outPublickeyStructs {
				//	delete(beaconBestState.RewardReceiver, value.GetIncKeyBase58())
				//}
				shardCommitteeStr, err := incognitokey.CommitteeKeyListToString(b.shardCommittee[shardID])
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				tempShardCommittees, err := RemoveValidator(shardCommitteeStr, swapInstruction.OutPublicKeys)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// remove old public key in shard committee update shard committee
				committeeChange.ShardCommitteeRemoved[shardID] = append(committeeChange.ShardCommitteeRemoved[shardID], swapInstruction.OutPublicKeyStructs...)
				b.shardCommittee[shardID], err = incognitokey.CommitteeBase58KeyListToStruct(tempShardCommittees)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// Check auto stake in out public keys list
				// if auto staking not found or flag auto stake is false then do not re-stake for this out public key
				// if auto staking flag is true then system will automatically add this out public key to current candidate list
				for index, outPublicKey := range swapInstruction.OutPublicKeys {
					if isAutoStaking, ok := b.autoStaking[outPublicKey]; !ok {
						if _, ok := b.rewardReceiver[outPublicKey]; ok {
							delete(b.rewardReceiver, swapInstruction.OutPublicKeyStructs[index].GetIncKeyBase58())
						}
						continue
					} else {
						if !isAutoStaking {
							// delete this flag for next time staking
							delete(b.rewardReceiver, swapInstruction.OutPublicKeyStructs[index].GetIncKeyBase58())
							delete(b.autoStaking, outPublicKey)
						} else {
							shardCandidate, err := incognitokey.CommitteeBase58KeyListToStruct([]string{outPublicKey})
							if err != nil {
								return newBeaconCandidates, newShardCandidates, err
							}
							newShardCandidates = append(newShardCandidates, shardCandidate...)
						}
					}
				}
			}
		} else {
			if len(swapInstruction.InPublicKeys) > 0 {
				beaconSubstituteStr, err := incognitokey.CommitteeKeyListToString(b.beaconSubstitute)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				tempBeaconSubstitute, err := RemoveValidator(beaconSubstituteStr, swapInstruction.InPublicKeys)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// update beacon pending validator
				committeeChange.BeaconSubstituteRemoved = append(committeeChange.BeaconSubstituteRemoved, swapInstruction.InPublicKeyStructs...)
				b.beaconSubstitute, err = incognitokey.CommitteeBase58KeyListToStruct(tempBeaconSubstitute)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// add new public key to beacon committee
				committeeChange.BeaconCommitteeAdded = append(committeeChange.BeaconCommitteeAdded, swapInstruction.InPublicKeyStructs...)
				b.beaconCommittee = append(b.beaconCommittee, swapInstruction.InPublicKeyStructs...)
			}
			if len(swapInstruction.OutPublicKeys) > 0 {
				// delete reward receiver
				//for _, value := range swapInstruction.OutPublicKeyStructs {
				//	delete(beaconBestState.RewardReceiver, value.GetIncKeyBase58())
				//}
				beaconCommitteeStrs, err := incognitokey.CommitteeKeyListToString(b.beaconCommittee)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				tempBeaconCommittees, err := RemoveValidator(beaconCommitteeStrs, swapInstruction.OutPublicKeys)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				// remove old public key in beacon committee and update beacon best state
				committeeChange.BeaconCommitteeRemoved = append(committeeChange.BeaconCommitteeRemoved, swapInstruction.OutPublicKeyStructs...)
				b.beaconCommittee, err = incognitokey.CommitteeBase58KeyListToStruct(tempBeaconCommittees)
				if err != nil {
					return newBeaconCandidates, newShardCandidates, err
				}
				for index, outPublicKey := range swapInstruction.OutPublicKeys {
					if isAutoStaking, ok := b.autoStaking[outPublicKey]; !ok {
						if _, ok := b.rewardReceiver[outPublicKey]; ok {
							delete(b.rewardReceiver, swapInstruction.OutPublicKeyStructs[index].GetIncKeyBase58())
						}
						continue
					} else {
						if !isAutoStaking {
							delete(b.rewardReceiver, swapInstruction.OutPublicKeyStructs[index].GetIncKeyBase58())
							delete(b.autoStaking, outPublicKey)
						} else {
							beaconCandidate, err := incognitokey.CommitteeBase58KeyListToStruct([]string{outPublicKey})
							if err != nil {
								return newBeaconCandidates, newShardCandidates, err
							}
							newBeaconCandidates = append(newBeaconCandidates, beaconCandidate...)
						}
					}
				}
			}
		}
	}
	return newBeaconCandidates, newShardCandidates, nil
}

func (b *BeaconCommitteeStateV1) processReplaceInstruction(
	swapInstruction *instruction.SwapInstruction,
	committeeChange *incognitokey.CommitteeChange,
) error {
	return nil
}

func (engine BeaconCommitteeEngine) ValidateCommitteeRootHashes(rootHashes []common.Hash) (bool, error) {
	panic("implement me")
}

func (engine BeaconCommitteeEngine) GetBeaconHeight() uint64 {
	return engine.beaconHeight
}
func (engine BeaconCommitteeEngine) GetBeaconHash() common.Hash {
	return engine.beaconHash
}

func (engine BeaconCommitteeEngine) GetBeaconCommittee() []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.beaconCommittee
}

func (engine BeaconCommitteeEngine) GetBeaconSubstitute() []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.beaconSubstitute
}

func (engine BeaconCommitteeEngine) GetCandidateShardWaitingForCurrentRandom() []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.currentEpochShardCandidate
}

func (engine BeaconCommitteeEngine) GetCandidateBeaconWaitingForCurrentRandom() []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.currentEpochBeaconCandidate
}

func (engine BeaconCommitteeEngine) GetCandidateShardWaitingForNextRandom() []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.nextEpochShardCandidate
}

func (engine BeaconCommitteeEngine) GetCandidateBeaconWaitingForNextRandom() []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.nextEpochBeaconCandidate
}

func (engine BeaconCommitteeEngine) GetShardCommittee(shardID byte) []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.shardCommittee[shardID]
}

func (engine BeaconCommitteeEngine) GetShardSubstitute(shardID byte) []incognitokey.CommitteePublicKey {
	return engine.beaconCommitteeStateV1.shardSubstitute[shardID]
}

func (engine BeaconCommitteeEngine) GetAutoStaking() map[string]bool {
	return engine.beaconCommitteeStateV1.autoStaking
}

func (engine BeaconCommitteeEngine) GetRewardReceiver() map[string]string {
	return engine.beaconCommitteeStateV1.rewardReceiver
}

func (b BeaconCommitteeStateV1) cloneBeaconCommitteeStateV1() *BeaconCommitteeStateV1 {
	newB := NewBeaconCommitteeStateV1()
	newB.beaconCommittee = b.beaconCommittee
	newB.beaconSubstitute = b.beaconSubstitute
	newB.currentEpochShardCandidate = b.currentEpochShardCandidate
	newB.currentEpochBeaconCandidate = b.currentEpochBeaconCandidate
	newB.nextEpochShardCandidate = b.nextEpochShardCandidate
	newB.nextEpochBeaconCandidate = b.nextEpochBeaconCandidate
	for k, v := range b.shardCommittee {
		newB.shardCommittee[k] = v
	}
	for k, v := range b.shardSubstitute {
		newB.shardSubstitute[k] = v
	}
	for k, v := range b.autoStaking {
		newB.autoStaking[k] = v
	}
	for k, v := range b.rewardReceiver {
		newB.rewardReceiver[k] = v
	}
	return newB
}

func (b *BeaconCommitteeStateV1) processAutoStakingChange(committeeChange *incognitokey.CommitteeChange) error {
	stopAutoStakingIncognitoKey, err := incognitokey.CommitteeBase58KeyListToStruct(committeeChange.StopAutoStake)
	if err != nil {
		return err
	}
	for _, committeePublicKey := range stopAutoStakingIncognitoKey {
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.NextEpochBeaconCandidateAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.CurrentEpochBeaconCandidateAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.NextEpochShardCandidateAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.CurrentEpochShardCandidateAdded) > -1 {
			continue
		}
		flag := false
		for _, v := range committeeChange.ShardSubstituteAdded {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				flag = true
				break
			}
		}
		if flag {
			continue
		}
		for _, v := range committeeChange.ShardCommitteeAdded {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				flag = true
				break
			}
		}
		if flag {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.BeaconSubstituteAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, committeeChange.BeaconCommitteeAdded) > -1 {
			continue
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, b.nextEpochBeaconCandidate) > -1 {
			committeeChange.NextEpochBeaconCandidateAdded = append(committeeChange.NextEpochBeaconCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, b.currentEpochBeaconCandidate) > -1 {
			committeeChange.CurrentEpochBeaconCandidateAdded = append(committeeChange.CurrentEpochBeaconCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, b.nextEpochShardCandidate) > -1 {
			committeeChange.NextEpochShardCandidateAdded = append(committeeChange.NextEpochShardCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, b.currentEpochShardCandidate) > -1 {
			committeeChange.CurrentEpochShardCandidateAdded = append(committeeChange.CurrentEpochShardCandidateAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, b.beaconSubstitute) > -1 {
			committeeChange.BeaconSubstituteAdded = append(committeeChange.BeaconSubstituteAdded, committeePublicKey)
		}
		if incognitokey.IndexOfCommitteeKey(committeePublicKey, b.beaconCommittee) > -1 {
			committeeChange.BeaconCommitteeAdded = append(committeeChange.BeaconCommitteeAdded, committeePublicKey)
		}
		for k, v := range b.shardCommittee {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				committeeChange.ShardCommitteeAdded[k] = append(committeeChange.ShardCommitteeAdded[k], committeePublicKey)
				flag = true
				break
			}
		}
		if flag {
			continue
		}
		for k, v := range b.shardSubstitute {
			if incognitokey.IndexOfCommitteeKey(committeePublicKey, v) > -1 {
				committeeChange.ShardSubstituteAdded[k] = append(committeeChange.ShardSubstituteAdded[k], committeePublicKey)
				flag = true
				break
			}
		}
		if flag {
			continue
		}
	}
	return nil
}
