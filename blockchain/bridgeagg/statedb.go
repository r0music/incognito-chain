package bridgeagg

import (
	"errors"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

func InitStateFromDB(sDB *statedb.StateDB, includeExternalTokenID bool) (*State, error) {
	unifiedTokenStates, err := statedb.GetBridgeAggUnifiedTokens(sDB)
	if err != nil {
		return nil, err
	}
	unifiedTokenInfos := make(map[common.Hash]map[uint]*Vault)
	for _, unifiedTokenState := range unifiedTokenStates {
		unifiedTokenInfos[unifiedTokenState.TokenID()] = make(map[uint]*Vault)
		convertTokens, err := statedb.GetBridgeAggConvertedTokens(sDB, unifiedTokenState.TokenID())
		if err != nil {
			return nil, err
		}
		for _, convertToken := range convertTokens {
			state, err := statedb.GetBridgeAggVault(sDB, unifiedTokenState.TokenID(), convertToken.TokenID())
			if err != nil {
				state = statedb.NewBridgeAggVaultState()
			}
			var externalTokenID []byte
			if includeExternalTokenID {
				info, has, err := statedb.GetBridgeTokenByType(sDB, convertToken.TokenID(), false)
				if err != nil {
					return nil, err
				}
				if !has {
					return nil, errors.New("Not found externalTokenID")
				}
				externalTokenID = info.ExternalTokenID()
			}
			vault := NewVaultWithValue(*state, convertToken.TokenID(), externalTokenID)
			unifiedTokenInfos[unifiedTokenState.TokenID()][convertToken.NetworkID()] = vault
		}
	}
	return NewStateWithValue(unifiedTokenInfos), nil
}
