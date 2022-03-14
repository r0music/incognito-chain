package bridgeagg

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	metadataBridgeAgg "github.com/incognitochain/incognito-chain/metadata/bridgeagg"
	metadataCommon "github.com/incognitochain/incognito-chain/metadata/common"
)

type stateProcessor struct {
}

func (sp *stateProcessor) modifyListTokens(
	inst metadataCommon.Instruction,
	unifiedTokenInfos map[common.Hash]map[uint]*Vault,
	sDB *statedb.StateDB,
) (map[common.Hash]map[uint]*Vault, error) {
	var status byte
	var txReqID common.Hash
	var errorCode uint
	switch inst.Status {
	case common.AcceptedStatusStr:
		contentBytes, err := base64.StdEncoding.DecodeString(inst.Content)
		if err != nil {
			return unifiedTokenInfos, err
		}
		acceptedInst := metadataBridgeAgg.AcceptedModifyListToken{}
		err = json.Unmarshal(contentBytes, &acceptedInst)
		if err != nil {
			return unifiedTokenInfos, err
		}
		for unifiedTokenID, vaults := range acceptedInst.NewListTokens {
			_, found := unifiedTokenInfos[unifiedTokenID]
			if !found {
				unifiedTokenInfos[unifiedTokenID] = make(map[uint]*Vault)
			}
			for _, vault := range vaults {
				if _, found := unifiedTokenInfos[unifiedTokenID][vault.NetworkID()]; !found {
					unifiedTokenInfos[unifiedTokenID][vault.NetworkID()] = NewVault()
				}
			}
		}
		txReqID = acceptedInst.TxReqID
		status = common.AcceptedStatusByte
	case common.RejectedStatusStr:
		rejectContent := metadataCommon.NewRejectContent()
		if err := rejectContent.FromString(inst.Content); err != nil {
			return unifiedTokenInfos, err
		}
		txReqID = rejectContent.TxReqID
		status = common.RejectedStatusByte
	default:
		return unifiedTokenInfos, errors.New("Can not recognize status")
	}
	modifyListTokenStatus := ModifyListTokenStatus{
		Status:    status,
		ErrorCode: errorCode,
	}
	contentBytes, _ := json.Marshal(modifyListTokenStatus)
	err := statedb.TrackBridgeAggStatus(
		sDB,
		statedb.BridgeAggListTokenModifyingStatusPrefix(),
		txReqID.Bytes(),
		contentBytes,
	)
	return unifiedTokenInfos, err
}

func (sp *stateProcessor) convert(
	inst metadataCommon.Instruction,
	unifiedTokenInfos map[common.Hash]map[uint]*Vault,
	sDB *statedb.StateDB,
) (map[common.Hash]map[uint]*Vault, error) {
	var status byte
	var txReqID common.Hash
	var errorCode uint
	switch inst.Status {
	case common.AcceptedStatusStr:
		contentBytes, err := base64.StdEncoding.DecodeString(inst.Content)
		if err != nil {
			return unifiedTokenInfos, err
		}
		acceptedInst := metadataBridgeAgg.AcceptedConvertTokenToUnifiedToken{}
		err = json.Unmarshal(contentBytes, &acceptedInst)
		if err != nil {
			return unifiedTokenInfos, err
		}
		if vaults, found := unifiedTokenInfos[acceptedInst.UnifiedTokenID]; found {
			if vault, found := vaults[acceptedInst.NetworkID]; found {
				vault.Convert(acceptedInst.Amount)
				unifiedTokenInfos[acceptedInst.UnifiedTokenID][acceptedInst.NetworkID] = vault
			} else {
				return unifiedTokenInfos, NewBridgeAggErrorWithValue(NotFoundTokenIDInNetwork, err)
			}
		} else {
			return unifiedTokenInfos, NewBridgeAggErrorWithValue(NotFoundTokenIDInNetwork, err)
		}
		txReqID = acceptedInst.TxReqID
		status = common.AcceptedStatusByte
	case common.RejectedStatusStr:
		rejectContent := metadataCommon.NewRejectContent()
		if err := rejectContent.FromString(inst.Content); err != nil {
			return unifiedTokenInfos, err
		}
		txReqID = rejectContent.TxReqID
		status = common.RejectedStatusByte
	default:
		return unifiedTokenInfos, errors.New("Can not recognize status")
	}
	convertStatus := ConvertStatus{
		Status:    status,
		ErrorCode: errorCode,
	}
	contentBytes, _ := json.Marshal(convertStatus)
	return unifiedTokenInfos, statedb.TrackBridgeAggStatus(
		sDB,
		statedb.BridgeAggConvertStatusPrefix(),
		txReqID.Bytes(),
		contentBytes,
	)
}
