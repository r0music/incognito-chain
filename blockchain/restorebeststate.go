package blockchain

import (
	"github.com/incognitochain/incognito-chain/blockchain/pdex"
	"github.com/incognitochain/incognito-chain/config"
)

//RestoreBeaconViewStateFromHash ...
func (beaconBestState *BeaconBestState) RestoreBeaconViewStateFromHash(
	blockchain *BlockChain, includeCommittee, includePdexv3 bool,
) error {
	err := beaconBestState.InitStateRootHash(blockchain)
	if err != nil {
		return err
	}
	//best block
	block, _, err := blockchain.GetBeaconBlockByHash(beaconBestState.BestBlockHash)
	if err != nil || block == nil {
		return err
	}
	beaconBestState.BestBlock = *block
	beaconBestState.BeaconHeight = block.GetHeight()
	beaconBestState.Epoch = block.GetCurrentEpoch()
	beaconBestState.BestBlockHash = *block.Hash()
	beaconBestState.PreviousBestBlockHash = block.GetPrevHash()

	if includeCommittee {
		err := beaconBestState.restoreCommitteeState(blockchain)
		if err != nil {
			return err
		}
	}

	if beaconBestState.BeaconHeight > config.Param().ConsensusParam.BlockProducingV3Height {
		if err := beaconBestState.checkBlockProducingV3Config(); err != nil {
			return err
		}
		if err := beaconBestState.upgradeBlockProducingV3Config(); err != nil {
			return err
		}
	}
	if includePdexv3 {
		beaconBestState.pdeStates = make(map[uint]pdex.State)
		pdeStates, ok := blockchain.beaconViewCache.Get(beaconBestState.BestBlockHash.String())
		if !ok || pdeStates == nil {
			state, err := pdex.InitStateFromDB(beaconBestState.GetBeaconFeatureStateDB(), beaconBestState.BeaconHeight, pdex.AmplifierVersion)
			if err != nil {
				return err
			}
			beaconBestState.pdeStates[pdex.AmplifierVersion] = state
			blockchain.pdeStatesCache.Add(beaconBestState.BestBlockHash.String(), beaconBestState.pdeStates)
		} else {
			beaconBestState.pdeStates = pdeStates.(map[uint]pdex.State)
		}
	}
	return nil
}
