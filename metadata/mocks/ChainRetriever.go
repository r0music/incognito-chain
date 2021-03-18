// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	common "github.com/incognitochain/incognito-chain/common"
	btcrelaying "github.com/incognitochain/incognito-chain/relaying/btc"

	metadata "github.com/incognitochain/incognito-chain/metadata"

	mock "github.com/stretchr/testify/mock"

	privacy "github.com/incognitochain/incognito-chain/privacy"
)

// ChainRetriever is an autogenerated mock type for the ChainRetriever type
type ChainRetriever struct {
	mock.Mock
}

// GetBCHeightBreakPointPortalV3 provides a mock function with given fields:
func (_m *ChainRetriever) GetBCHeightBreakPointPortalV3() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetBNBChainID provides a mock function with given fields:
func (_m *ChainRetriever) GetBNBChainID() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetBTCChainID provides a mock function with given fields:
func (_m *ChainRetriever) GetBTCChainID() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetBTCHeaderChain provides a mock function with given fields:
func (_m *ChainRetriever) GetBTCHeaderChain() *btcrelaying.BlockChain {
	ret := _m.Called()

	var r0 *btcrelaying.BlockChain
	if rf, ok := ret.Get(0).(func() *btcrelaying.BlockChain); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*btcrelaying.BlockChain)
		}
	}

	return r0
}

// GetBeaconHeightBreakPointBurnAddr provides a mock function with given fields:
func (_m *ChainRetriever) GetBeaconHeightBreakPointBurnAddr() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetBurningAddress provides a mock function with given fields: blockHeight
func (_m *ChainRetriever) GetBurningAddress(blockHeight uint64) string {
	ret := _m.Called(blockHeight)

	var r0 string
	if rf, ok := ret.Get(0).(func(uint64) string); ok {
		r0 = rf(blockHeight)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetCentralizedWebsitePaymentAddress provides a mock function with given fields: _a0
func (_m *ChainRetriever) GetCentralizedWebsitePaymentAddress(_a0 uint64) string {
	ret := _m.Called(_a0)

	var r0 string
	if rf, ok := ret.Get(0).(func(uint64) string); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetETHRemoveBridgeSigEpoch provides a mock function with given fields:
func (_m *ChainRetriever) GetETHRemoveBridgeSigEpoch() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetFixedRandomForShardIDCommitment provides a mock function with given fields: beaconHeight
func (_m *ChainRetriever) GetFixedRandomForShardIDCommitment(beaconHeight uint64) *privacy.Scalar {
	ret := _m.Called(beaconHeight)

	var r0 *privacy.Scalar
	if rf, ok := ret.Get(0).(func(uint64) *privacy.Scalar); ok {
		r0 = rf(beaconHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*privacy.Scalar)
		}
	}

	return r0
}

// GetPortalETHContractAddrStr provides a mock function with given fields:
func (_m *ChainRetriever) GetPortalETHContractAddrStr() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetPortalFeederAddress provides a mock function with given fields:
func (_m *ChainRetriever) GetPortalFeederAddress() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// GetShardStakingTx provides a mock function with given fields: shardID, beaconHeight
func (_m *ChainRetriever) GetShardStakingTx(shardID byte, beaconHeight uint64) (map[string]string, error) {
	ret := _m.Called(shardID, beaconHeight)

	var r0 map[string]string
	if rf, ok := ret.Get(0).(func(byte, uint64) map[string]string); ok {
		r0 = rf(shardID, beaconHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(byte, uint64) error); ok {
		r1 = rf(shardID, beaconHeight)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetStakingAmountShard provides a mock function with given fields:
func (_m *ChainRetriever) GetStakingAmountShard() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetSupportedCollateralTokenIDs provides a mock function with given fields: beaconHeight
func (_m *ChainRetriever) GetSupportedCollateralTokenIDs(beaconHeight uint64) []string {
	ret := _m.Called(beaconHeight)

	var r0 []string
	if rf, ok := ret.Get(0).(func(uint64) []string); ok {
		r0 = rf(beaconHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}

// GetTransactionByHash provides a mock function with given fields: _a0
func (_m *ChainRetriever) GetTransactionByHash(_a0 common.Hash) (byte, common.Hash, uint64, int, metadata.Transaction, error) {
	ret := _m.Called(_a0)

	var r0 byte
	if rf, ok := ret.Get(0).(func(common.Hash) byte); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(byte)
	}

	var r1 common.Hash
	if rf, ok := ret.Get(1).(func(common.Hash) common.Hash); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(common.Hash)
		}
	}

	var r2 uint64
	if rf, ok := ret.Get(2).(func(common.Hash) uint64); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Get(2).(uint64)
	}

	var r3 int
	if rf, ok := ret.Get(3).(func(common.Hash) int); ok {
		r3 = rf(_a0)
	} else {
		r3 = ret.Get(3).(int)
	}

	var r4 metadata.Transaction
	if rf, ok := ret.Get(4).(func(common.Hash) metadata.Transaction); ok {
		r4 = rf(_a0)
	} else {
		if ret.Get(4) != nil {
			r4 = ret.Get(4).(metadata.Transaction)
		}
	}

	var r5 error
	if rf, ok := ret.Get(5).(func(common.Hash) error); ok {
		r5 = rf(_a0)
	} else {
		r5 = ret.Error(5)
	}

	return r0, r1, r2, r3, r4, r5
}

// ListPrivacyTokenAndBridgeTokenAndPRVByShardID provides a mock function with given fields: _a0
func (_m *ChainRetriever) ListPrivacyTokenAndBridgeTokenAndPRVByShardID(_a0 byte) ([]common.Hash, error) {
	ret := _m.Called(_a0)

	var r0 []common.Hash
	if rf, ok := ret.Get(0).(func(byte) []common.Hash); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]common.Hash)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(byte) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
