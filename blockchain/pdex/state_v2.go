package pdex

import (
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/config"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
	metadataPdexv3 "github.com/incognitochain/incognito-chain/metadata/pdexv3"
	"github.com/incognitochain/incognito-chain/utils"
)

type stateV2 struct {
	stateBase
	waitingContributions        map[string]rawdbv2.Pdexv3Contribution
	deletedWaitingContributions map[string]rawdbv2.Pdexv3Contribution
	poolPairs                   map[string]*PoolPairState
	params                      *Params
	stakingPoolsState           map[string]*StakingPoolState // tokenID -> StakingPoolState
	nftIDs                      map[string]uint64
	producer                    stateProducerV2
	processor                   stateProcessorV2
}

func newStateV2() *stateV2 {
	return &stateV2{
		params:                      NewParams(),
		waitingContributions:        make(map[string]rawdbv2.Pdexv3Contribution),
		deletedWaitingContributions: make(map[string]rawdbv2.Pdexv3Contribution),
		poolPairs:                   make(map[string]*PoolPairState),
		stakingPoolsState:           make(map[string]*StakingPoolState),
		nftIDs:                      make(map[string]uint64),
	}
}

func newStateV2WithValue(
	waitingContributions map[string]rawdbv2.Pdexv3Contribution,
	deletedWaitingContributions map[string]rawdbv2.Pdexv3Contribution,
	poolPairs map[string]*PoolPairState,
	params *Params,
	stakingPoolsState map[string]*StakingPoolState,
	nftIDs map[string]uint64,
) *stateV2 {
	return &stateV2{
		waitingContributions:        waitingContributions,
		deletedWaitingContributions: deletedWaitingContributions,
		poolPairs:                   poolPairs,
		stakingPoolsState:           stakingPoolsState,
		params:                      params,
		nftIDs:                      nftIDs,
	}
}

func initStateV2(
	stateDB *statedb.StateDB,
	beaconHeight uint64,
) (*stateV2, error) {
	paramsState, err := statedb.GetPdexv3Params(stateDB)
	params := NewParamsWithValue(paramsState)
	if err != nil {
		return nil, err
	}
	waitingContributions, err := statedb.GetPdexv3WaitingContributions(stateDB)
	if err != nil {
		return nil, err
	}
	poolPairsStates, err := statedb.GetPdexv3PoolPairs(stateDB)
	if err != nil {
		return nil, err
	}
	poolPairs := make(map[string]*PoolPairState)
	for poolPairID, poolPairState := range poolPairsStates {
		shares := make(map[string]*Share)
		shareStates := make(map[string]statedb.Pdexv3ShareState)
		shareStates, err = statedb.GetPdexv3Shares(stateDB, poolPairID)
		if err != nil {
			return nil, err
		}
		for nftID, shareState := range shareStates {
			shares[nftID] = NewShareWithValue(
				shareState.Amount(),
				shareState.TradingFees(), shareState.LastLPFeesPerShare(),
			)
		}

		orderbook := &Orderbook{[]*Order{}}
		orderMap, err := statedb.GetPdexv3Orders(stateDB, poolPairState.PoolPairID())
		if err != nil {
			return nil, err
		}
		for _, item := range orderMap {
			v := item.Value()
			orderbook.InsertOrder(&v)
		}
		poolPair := NewPoolPairStateWithValue(
			poolPairState.Value(), shares, *orderbook,
		)
		poolPairs[poolPairID] = poolPair
	}

	nftIDs, err := statedb.GetPdexv3NftIDs(stateDB)
	if err != nil {
		return nil, err
	}
	return newStateV2WithValue(
		waitingContributions, make(map[string]rawdbv2.Pdexv3Contribution),
		poolPairs, params, nil, nftIDs,
	), nil
}

func (s *stateV2) Version() uint {
	return AmplifierVersion
}

func (s *stateV2) Clone() State {
	res := newStateV2()
	res.params = s.params.Clone()

	for k, v := range s.stakingPoolsState {
		res.stakingPoolsState[k] = v.Clone()
	}
	for k, v := range s.waitingContributions {
		res.waitingContributions[k] = *v.Clone()
	}
	for k, v := range s.deletedWaitingContributions {
		res.deletedWaitingContributions[k] = *v.Clone()
	}
	for k, v := range s.poolPairs {
		res.poolPairs[k] = v.Clone()
	}
	for k, v := range s.nftIDs {
		res.nftIDs[k] = v
	}
	res.producer = s.producer
	res.processor = s.processor

	return res
}

func (s *stateV2) Process(env StateEnvironment) error {
	s.processor.clearCache()
	for _, inst := range env.BeaconInstructions() {
		if len(inst) < 2 {
			continue // Not error, just not PDE instructions
		}
		metadataType, err := strconv.Atoi(inst[0])
		if err != nil {
			continue // Not error, just not PDE instructions
		}
		if !metadataCommon.IsPdexv3Type(metadataType) {
			continue // Not error, just not PDE instructions
		}
		switch metadataType {
		case metadataCommon.Pdexv3MintPDEXBlockRewardMeta:
			s.poolPairs, err = s.processor.mintPDEX(
				env.StateDB(),
				inst,
				s.poolPairs,
			)
		case metadataCommon.Pdexv3UserMintNftRequestMeta:
			s.nftIDs, _, err = s.processor.userMintNft(env.StateDB(), inst, s.nftIDs)
			if err != nil {
				Logger.log.Debugf("process inst %s err %v:", inst, err)
				continue
			}
		case metadataCommon.Pdexv3AddLiquidityRequestMeta:
			s.poolPairs,
				s.waitingContributions,
				s.deletedWaitingContributions, err = s.processor.addLiquidity(
				env.StateDB(),
				inst,
				env.BeaconHeight(),
				s.poolPairs,
				s.waitingContributions, s.deletedWaitingContributions,
			)
		case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
			s.poolPairs, err = s.processor.withdrawLiquidity(env.StateDB(), inst, s.poolPairs)
		case metadataCommon.Pdexv3TradeRequestMeta:
			s.poolPairs, err = s.processor.trade(env.StateDB(), inst,
				s.poolPairs,
			)
		case metadataCommon.Pdexv3WithdrawLPFeeRequestMeta:
			s.poolPairs, err = s.processor.withdrawLPFee(
				env.StateDB(),
				inst,
				env.BeaconHeight(),
				s.poolPairs,
			)
		case metadataCommon.Pdexv3WithdrawProtocolFeeRequestMeta:
			s.poolPairs, err = s.processor.withdrawProtocolFee(
				env.StateDB(),
				inst,
				s.poolPairs,
			)
		case metadataCommon.Pdexv3AddOrderRequestMeta:
			s.poolPairs, err = s.processor.addOrder(env.StateDB(), inst,
				s.poolPairs,
			)
		case metadataCommon.Pdexv3WithdrawOrderRequestMeta:
			s.poolPairs, err = s.processor.withdrawOrder(env.StateDB(), inst,
				s.poolPairs,
			)
		case metadataCommon.Pdexv3ModifyParamsMeta:
			s.params, err = s.processor.modifyParams(
				env.StateDB(),
				inst,
				s.params,
			)
		default:
			Logger.log.Debug("Can not process this metadata")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *stateV2) BuildInstructions(env StateEnvironment) ([][]string, error) {
	instructions := [][]string{}
	addLiquidityTxs := []metadata.Transaction{}
	withdrawLPFeeTxs := []metadata.Transaction{}
	withdrawlProtocolFeeTxs := []metadata.Transaction{}
	withdrawLiquidityTxs := []metadata.Transaction{}
	modifyParamsTxs := []metadata.Transaction{}
	tradeTxs := []metadata.Transaction{}
	mintNftTxs := []metadata.Transaction{}
	addOrderTxs := []metadata.Transaction{}
	withdrawOrderTxs := []metadata.Transaction{}

	var err error
	pdexv3Txs := env.ListTxs()
	keys := []int{}

	for k := range pdexv3Txs {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, key := range keys {
		for _, tx := range pdexv3Txs[byte(key)] {
			switch tx.GetMetadataType() {
			case metadataCommon.Pdexv3UserMintNftRequestMeta:
				mintNftTxs = append(mintNftTxs, tx)
			case metadataCommon.Pdexv3AddLiquidityRequestMeta:
				addLiquidityTxs = append(addLiquidityTxs, tx)
			case metadataCommon.Pdexv3WithdrawLiquidityRequestMeta:
				withdrawLiquidityTxs = append(withdrawLiquidityTxs, tx)
			case metadataCommon.Pdexv3ModifyParamsMeta:
				modifyParamsTxs = append(modifyParamsTxs, tx)
			case metadataCommon.Pdexv3TradeRequestMeta:
				tradeTxs = append(tradeTxs, tx)
			case metadataCommon.Pdexv3WithdrawLPFeeRequestMeta:
				withdrawLPFeeTxs = append(withdrawLPFeeTxs, tx)
			case metadataCommon.Pdexv3WithdrawProtocolFeeRequestMeta:
				withdrawlProtocolFeeTxs = append(withdrawlProtocolFeeTxs, tx)
			case metadataCommon.Pdexv3AddOrderRequestMeta:
				addOrderTxs = append(addOrderTxs, tx)
			case metadataCommon.Pdexv3WithdrawOrderRequestMeta:
				withdrawOrderTxs = append(withdrawOrderTxs, tx)
			}
		}
	}

	mintNftInstructions := [][]string{}
	mintNftInstructions, s.nftIDs, err = s.producer.userMintNft(mintNftTxs, s.nftIDs, env.BeaconHeight(), s.params.MintNftRequireAmount)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, mintNftInstructions...)

	withdrawLiquidityInstructions := [][]string{}
	withdrawLiquidityInstructions, s.poolPairs, err = s.producer.withdrawLiquidity(withdrawLiquidityTxs, s.poolPairs, s.nftIDs)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, withdrawLiquidityInstructions...)

	addLiquidityInstructions := [][]string{}
	addLiquidityInstructions, s.poolPairs, s.waitingContributions, err = s.producer.addLiquidity(
		addLiquidityTxs,
		env.BeaconHeight(),
		s.poolPairs,
		s.waitingContributions,
		s.nftIDs,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, addLiquidityInstructions...)

	pdexBlockRewards := uint64(0)
	// mint PDEX token at the pDex v3 checkpoint block
	if env.BeaconHeight() == config.Param().PDexParams.Pdexv3BreakPointHeight {
		mintPDEXGenesis, err := s.producer.mintPDEXGenesis()
		if err != nil {
			return instructions, err
		}
		instructions = append(instructions, mintPDEXGenesis...)
	} else if env.BeaconHeight() > config.Param().PDexParams.Pdexv3BreakPointHeight {
		intervalLength := uint64(MintingBlocks / DecayIntervals)
		decayIntevalIdx := (env.BeaconHeight() - config.Param().PDexParams.Pdexv3BreakPointHeight) / intervalLength
		if decayIntevalIdx < DecayIntervals {
			curIntervalReward := PDEXRewardFirstInterval
			for i := uint64(0); i < decayIntevalIdx; i++ {
				curIntervalReward -= curIntervalReward * DecayRateBPS / BPS
			}
			pdexBlockRewards = curIntervalReward / intervalLength
		}
	}

	if pdexBlockRewards > 0 {
		var mintInstructions [][]string
		mintInstructions, s.poolPairs, err = s.producer.mintPDEX(
			pdexBlockRewards,
			s.params,
			s.poolPairs,
		)
		if err != nil {
			return instructions, err
		}
		instructions = append(instructions, mintInstructions...)
	}

	var tradeInstructions [][]string
	tradeInstructions, s.poolPairs, err = s.producer.trade(
		tradeTxs,
		s.poolPairs,
		s.params,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, tradeInstructions...)

	var withdrawLPFeeInstructions [][]string
	withdrawLPFeeInstructions, s.poolPairs, err = s.producer.withdrawLPFee(
		withdrawLPFeeTxs,
		s.poolPairs,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, withdrawLPFeeInstructions...)

	var withdrawProtocolFeeInstructions [][]string
	withdrawProtocolFeeInstructions, s.poolPairs, err = s.producer.withdrawProtocolFee(
		withdrawlProtocolFeeTxs,
		s.poolPairs,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, withdrawProtocolFeeInstructions...)

	var addOrderInstructions [][]string
	addOrderInstructions, s.poolPairs, err = s.producer.addOrder(
		addOrderTxs,
		s.poolPairs,
		s.nftIDs,
		s.params,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, addOrderInstructions...)

	var withdrawOrderInstructions [][]string
	withdrawOrderInstructions, s.poolPairs, err = s.producer.withdrawOrder(
		withdrawOrderTxs,
		s.poolPairs,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, withdrawOrderInstructions...)

	// handle modify params: at the end of beacon block
	var modifyParamsInstructions [][]string
	modifyParamsInstructions, s.params, err = s.producer.modifyParams(
		modifyParamsTxs,
		env.BeaconHeight(),
		s.params,
		s.poolPairs,
		s.stakingPoolsState,
	)
	if err != nil {
		return instructions, err
	}
	instructions = append(instructions, modifyParamsInstructions...)

	return instructions, nil
}

func (s *stateV2) Upgrade(env StateEnvironment) State {
	return nil
}

func (s *stateV2) StoreToDB(env StateEnvironment, stateChange *StateChange) error {
	err := statedb.StorePdexv3Params(
		env.StateDB(),
		s.params.DefaultFeeRateBPS,
		s.params.FeeRateBPS,
		s.params.PRVDiscountPercent,
		s.params.TradingProtocolFeePercent,
		s.params.TradingStakingPoolRewardPercent,
		s.params.PDEXRewardPoolPairsShare,
		s.params.StakingPoolsShare,
		s.params.StakingRewardTokens,
		s.params.MintNftRequireAmount,
	)
	if err != nil {
		return err
	}
	deletedWaitingContributionsKeys := []string{}
	for k := range s.deletedWaitingContributions {
		deletedWaitingContributionsKeys = append(deletedWaitingContributionsKeys, k)
	}
	err = statedb.DeletePdexv3WaitingContributions(env.StateDB(), deletedWaitingContributionsKeys)
	if err != nil {
		return err
	}
	err = statedb.StorePdexv3WaitingContributions(env.StateDB(), s.waitingContributions)
	if err != nil {
		return err
	}
	for poolPairID, poolPairState := range s.poolPairs {
		if stateChange.poolPairIDs[poolPairID] {
			err := statedb.StorePdexv3PoolPair(env.StateDB(), poolPairID, poolPairState.state)
			if err != nil {
				return err
			}
		}
		for nftID, share := range poolPairState.shares {
			if stateChange.shares[nftID] {

				nftID, err := common.Hash{}.NewHashFromStr(nftID)
				err = statedb.StorePdexv3Share(
					env.StateDB(), poolPairID,
					*nftID,
					share.amount, share.tradingFees, share.lastLPFeesPerShare,
				)
				if err != nil {
					return err
				}
			}
		}
	}
	err = statedb.StorePdexv3NftIDs(env.StateDB(), s.nftIDs)
	if err != nil {
		return err
	}
	return statedb.StorePdexv3StakingPools()
}

func (s *stateV2) ClearCache() {
	s.deletedWaitingContributions = make(map[string]rawdbv2.Pdexv3Contribution)
}

func (s *stateV2) GetDiff(compareState State, stateChange *StateChange) (State, *StateChange, error) {
	newStateChange := stateChange
	if compareState == nil {
		return nil, newStateChange, errors.New("compareState is nil")
	}

	res := newStateV2()
	compareStateV2 := compareState.(*stateV2)

	res.params = s.params
	clonedFeeRateBPS := map[string]uint{}
	for k, v := range s.params.FeeRateBPS {
		clonedFeeRateBPS[k] = v
	}
	clonedPDEXRewardPoolPairsShare := map[string]uint{}
	for k, v := range s.params.PDEXRewardPoolPairsShare {
		clonedPDEXRewardPoolPairsShare[k] = v
	}
	clonedStakingPoolsShare := map[string]uint{}
	for k, v := range s.params.StakingPoolsShare {
		clonedStakingPoolsShare[k] = v
	}
	res.params.FeeRateBPS = clonedFeeRateBPS
	res.params.PDEXRewardPoolPairsShare = clonedPDEXRewardPoolPairsShare
	res.params.StakingPoolsShare = clonedStakingPoolsShare

	for k, v := range s.waitingContributions {
		if m, ok := compareStateV2.waitingContributions[k]; !ok || !reflect.DeepEqual(m, v) {
			res.waitingContributions[k] = *v.Clone()
		}
	}
	for k, v := range s.deletedWaitingContributions {
		if m, ok := compareStateV2.deletedWaitingContributions[k]; !ok || !reflect.DeepEqual(m, v) {
			res.deletedWaitingContributions[k] = *v.Clone()
		}
	}
	for k, v := range s.poolPairs {
		if m, ok := compareStateV2.poolPairs[k]; !ok || !reflect.DeepEqual(m, v) {
			newStateChange = v.getDiff(k, m, newStateChange)
			res.poolPairs[k] = v.Clone()
		}
	}
	for k, v := range s.stakingPoolsState {
		if m, ok := compareStateV2.stakingPoolsState[k]; !ok || !reflect.DeepEqual(m, v) {
			res.stakingPoolsState[k] = v.Clone()
		}
	}
	for k, v := range s.nftIDs {
		if m, ok := compareStateV2.nftIDs[k]; !ok || !reflect.DeepEqual(m, v) {
			res.nftIDs[k] = v
		}
	}

	return res, newStateChange, nil

}

func (s *stateV2) Params() *Params {
	return s.params
}

func (s *stateV2) Reader() StateReader {
	return s
}

func NewContributionWithMetaData(
	metaData metadataPdexv3.AddLiquidityRequest, txReqID common.Hash, shardID byte,
) *rawdbv2.Pdexv3Contribution {
	tokenHash, _ := common.Hash{}.NewHashFromStr(metaData.TokenID())
	nftID := common.Hash{}
	if metaData.NftID() != utils.EmptyString {
		nftHash, _ := common.Hash{}.NewHashFromStr(metaData.NftID())
		nftID = *nftHash
	}
	return rawdbv2.NewPdexv3ContributionWithValue(
		metaData.PoolPairID(), metaData.OtaReceive(), metaData.OtaRefund(),
		*tokenHash, txReqID, nftID,
		metaData.TokenAmount(), metaData.Amplifier(),
		shardID,
	)
}

func (s *stateV2) WaitingContributions() []byte {
	temp := make(map[string]*rawdbv2.Pdexv3Contribution, len(s.waitingContributions))
	for k, v := range s.waitingContributions {
		temp[k] = v.Clone()
	}
	data, _ := json.Marshal(temp)
	return data
}

func (s *stateV2) PoolPairs() []byte {
	temp := make(map[string]*PoolPairState, len(s.poolPairs))
	for k, v := range s.poolPairs {
		temp[k] = v.Clone()
	}
	data, _ := json.Marshal(temp)
	return data
}

func (s *stateV2) TransformKeyWithNewBeaconHeight(beaconHeight uint64) {}

func (s *stateV2) NftIDs() map[string]uint64 {
	res := make(map[string]uint64)
	for k, v := range s.nftIDs {
		res[k] = v
	}
	return res
}
