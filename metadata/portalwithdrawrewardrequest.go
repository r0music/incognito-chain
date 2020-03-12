package metadata

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database"
	"github.com/incognitochain/incognito-chain/wallet"
	"strconv"
)

// PortalRequestWithdrawReward - custodians request withdraw reward
// metadata - custodians request withdraw reward - create normal tx with this metadata
type PortalRequestWithdrawReward struct {
	MetadataBase
	CustodianAddressStr string
}

// PortalRequestWithdrawRewardAction - shard validator creates instruction that contain this action content
// it will be append to ShardToBeaconBlock
type PortalRequestWithdrawRewardAction struct {
	Meta    PortalRequestWithdrawReward
	TxReqID common.Hash
	ShardID byte
}

// PortalRequestWithdrawRewardContent - Beacon builds a new instruction with this content after receiving a instruction from shard
// It will be appended to beaconBlock
// both accepted and rejected status
type PortalRequestWithdrawRewardContent struct {
	CustodianAddressStr string
	RewardAmount        uint64
	TxReqID             common.Hash
	ShardID             byte
}

// PortalRequestWithdrawRewardStatus - Beacon tracks status of request unlock collateral amount into db
type PortalRequestWithdrawRewardStatus struct {
	Status              byte
	CustodianAddressStr string
	RewardAmount        uint64
	TxReqID             common.Hash
}

func NewPortalRequestWithdrawReward(
	metaType int,
	incogAddressStr string,) (*PortalRequestWithdrawReward, error) {
	metadataBase := MetadataBase{
		Type: metaType,
	}
	meta := &PortalRequestWithdrawReward{

		CustodianAddressStr: incogAddressStr,
	}
	meta.MetadataBase = metadataBase
	return meta, nil
}

func (meta PortalRequestWithdrawReward) ValidateTxWithBlockChain(
	txr Transaction,
	bcr BlockchainRetriever,
	shardID byte,
	db database.DatabaseInterface,
) (bool, error) {
	return true, nil
}

func (meta PortalRequestWithdrawReward) ValidateSanityData(bcr BlockchainRetriever, txr Transaction, beaconHeight uint64) (bool, bool, error) {
	// validate CustodianAddressStr
	keyWallet, err := wallet.Base58CheckDeserialize(meta.CustodianAddressStr)
	if err != nil {
		return false, false, errors.New("Custodian incognito address is invalid")
	}
	incogAddr := keyWallet.KeySet.PaymentAddress
	if len(incogAddr.Pk) == 0 {
		return false, false, errors.New("Custodian incognito address is invalid")
	}
	if !bytes.Equal(txr.GetSigPubKey()[:], incogAddr.Pk[:]) {
		return false, false,  errors.New("Custodian incognito address is not signer")
	}

	// check tx type
	if txr.GetType() != common.TxNormalType {
		return false, false, errors.New("tx request withdraw reward must be TxNormalType")
	}

	return true, true, nil
}

func (meta PortalRequestWithdrawReward) ValidateMetadataByItself() bool {
	return meta.Type == PortalRequestUnlockCollateralMeta
}

func (meta PortalRequestWithdrawReward) Hash() *common.Hash {
	record := meta.MetadataBase.Hash().String()
	record += meta.CustodianAddressStr
	// final hash
	hash := common.HashH([]byte(record))
	return &hash
}

func (meta *PortalRequestWithdrawReward) BuildReqActions(tx Transaction, bcr BlockchainRetriever, shardID byte) ([][]string, error) {
	actionContent := PortalRequestWithdrawRewardAction{
		Meta:    *meta,
		TxReqID: *tx.Hash(),
		ShardID: shardID,
	}
	actionContentBytes, err := json.Marshal(actionContent)
	if err != nil {
		return [][]string{}, err
	}
	actionContentBase64Str := base64.StdEncoding.EncodeToString(actionContentBytes)
	action := []string{strconv.Itoa(PortalRequestWithdrawRewardMeta), actionContentBase64Str}
	return [][]string{action}, nil
}

func (meta *PortalRequestWithdrawReward) CalculateSize() uint64 {
	return calculateSize(meta)
}



