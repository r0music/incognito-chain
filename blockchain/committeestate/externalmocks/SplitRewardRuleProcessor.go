// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package externalmocks

import (
	committeestate "github.com/incognitochain/incognito-chain/blockchain/committeestate"
	common "github.com/incognitochain/incognito-chain/common"

	mock "github.com/stretchr/testify/mock"
)

// SplitRewardRuleProcessor is an autogenerated mock type for the SplitRewardRuleProcessor type
type SplitRewardRuleProcessor struct {
	mock.Mock
}

// SplitReward provides a mock function with given fields: environment
func (_m *SplitRewardRuleProcessor) SplitReward(environment *committeestate.SplitRewardEnvironment) (map[common.Hash]uint64, map[common.Hash]uint64, map[common.Hash]uint64, map[common.Hash]uint64, error) {
	ret := _m.Called(environment)

	var r0 map[common.Hash]uint64
	if rf, ok := ret.Get(0).(func(*committeestate.SplitRewardEnvironment) map[common.Hash]uint64); ok {
		r0 = rf(environment)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[common.Hash]uint64)
		}
	}

	var r1 map[common.Hash]uint64
	if rf, ok := ret.Get(1).(func(*committeestate.SplitRewardEnvironment) map[common.Hash]uint64); ok {
		r1 = rf(environment)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(map[common.Hash]uint64)
		}
	}

	var r2 map[common.Hash]uint64
	if rf, ok := ret.Get(2).(func(*committeestate.SplitRewardEnvironment) map[common.Hash]uint64); ok {
		r2 = rf(environment)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(map[common.Hash]uint64)
		}
	}

	var r3 map[common.Hash]uint64
	if rf, ok := ret.Get(3).(func(*committeestate.SplitRewardEnvironment) map[common.Hash]uint64); ok {
		r3 = rf(environment)
	} else {
		if ret.Get(3) != nil {
			r3 = ret.Get(3).(map[common.Hash]uint64)
		}
	}

	var r4 error
	if rf, ok := ret.Get(4).(func(*committeestate.SplitRewardEnvironment) error); ok {
		r4 = rf(environment)
	} else {
		r4 = ret.Error(4)
	}

	return r0, r1, r2, r3, r4
}
