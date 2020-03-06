package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database"
	"github.com/incognitochain/incognito-chain/wallet"
	"strconv"
)

type PortalCustodianWithdrawResponse struct {
	MetadataBase
	RequestStatus	string
	ReqTxID        	common.Hash
	PaymentAddress	string
	Amount         	uint64
}

func NewPortalCustodianWithdrawResponse(
	requestStatus string,
	reqTxId common.Hash,
	paymentAddress string,
	amount uint64,
	metaType int,
) *PortalCustodianWithdrawResponse {
	metaDataBase := MetadataBase{Type:metaType}

	return &PortalCustodianWithdrawResponse{
		MetadataBase: metaDataBase,
		RequestStatus: requestStatus,
		ReqTxID: reqTxId,
		PaymentAddress: paymentAddress,
		Amount: amount,
	}
}

func (responseMeta PortalCustodianWithdrawResponse) CheckTransactionFee(tr Transaction, minFee uint64, beaconHeight int64, db database.DatabaseInterface) bool {
	// no need to have fee for this tx
	return true
}

func (responseMeta PortalCustodianWithdrawResponse) ValidateTxWithBlockChain(txr Transaction, bcr BlockchainRetriever, shardID byte, db database.DatabaseInterface) (bool, error) {
	// no need to validate tx with blockchain, just need to validate with requested tx (via RequestedTxID)
	return false, nil
}

func (responseMeta PortalCustodianWithdrawResponse) ValidateSanityData(bcr BlockchainRetriever, txr Transaction, beaconHeight uint64) (bool, bool, error) {
	return false, true, nil
}

func (responseMeta PortalCustodianWithdrawResponse) ValidateMetadataByItself() bool {
	// The validation just need to check at tx level, so returning true here
	return responseMeta.Type == PortalCustodianWithdrawResponseMeta
}

func (responseMeta PortalCustodianWithdrawResponse) Hash() *common.Hash {
	record := responseMeta.MetadataBase.Hash().String()
	record += responseMeta.RequestStatus
	record += responseMeta.ReqTxID.String()
	record += responseMeta.PaymentAddress
	record += strconv.FormatUint(responseMeta.Amount, 10)
	// final hash
	hash := common.HashH([]byte(record))
	return &hash
}

func (responseMeta *PortalCustodianWithdrawResponse) CalculateSize() uint64 {
	return calculateSize(responseMeta)
}

//todo: refactor
func (responseMeta PortalCustodianWithdrawResponse) VerifyMinerCreatedTxBeforeGettingInBlock(
	txsInBlock []Transaction,
	txsUsed []int,
	insts [][]string,
	instUsed []int,
	shardID byte,
	tx Transaction,
	bcr BlockchainRetriever,
	ac *AccumulatedValues,
) (bool, error) {
	idx := -1
	for i, inst := range insts {
		if len(inst) < 4 { // this is not PortalRequestPTokens response instruction
			continue
		}
		instMetaType := inst[0]
		if instUsed[i] > 0 ||
			instMetaType != strconv.Itoa(PortalCustodianWithdrawRequestMeta) {
			continue
		}
		instDepositStatus := inst[2]
		if instDepositStatus != responseMeta.RequestStatus ||
			(instDepositStatus != common.PortalCustodianWithdrawRequestAcceptedStatus) {
			continue
		}

		var shardIDFromInst byte
		var txReqIDFromInst common.Hash
		var requesterAddrStrFromInst string
		var portingAmountFromInst uint64

		contentBytes := []byte(inst[3])
		var custodianWithdrawRequest PortalCustodianWithdrawRequestContent
		err := json.Unmarshal(contentBytes, &custodianWithdrawRequest)
		if err != nil {
			Logger.log.Error("WARNING - VALIDATION: an error occured while parsing custodian withdraw request content: ", err)
			continue
		}
		shardIDFromInst = custodianWithdrawRequest.ShardID
		txReqIDFromInst = custodianWithdrawRequest.TxReqID
		requesterAddrStrFromInst = custodianWithdrawRequest.PaymentAddress
		portingAmountFromInst = custodianWithdrawRequest.Amount
		receivingTokenIDStr := common.PRVCoinID.String()

		if !bytes.Equal(responseMeta.ReqTxID[:], txReqIDFromInst[:]) ||
			shardID != shardIDFromInst {
			continue
		}

		key, err := wallet.Base58CheckDeserialize(requesterAddrStrFromInst)
		if err != nil {
			Logger.log.Info("WARNING - VALIDATION: an error occured while deserializing receiver address string: ", err)
			continue
		}

		_, pk, amount, assetID := tx.GetTransferData()
		if !bytes.Equal(key.KeySet.PaymentAddress.Pk[:], pk[:]) ||
			portingAmountFromInst != amount || receivingTokenIDStr != assetID.String() {
			continue
		}

		idx = i
		break
	}

	if idx == -1 { // not found the issuance request tx for this response
		return false, fmt.Errorf(fmt.Sprintf("no PortalReqPtokens instruction found for PortalReqPtokensResponse tx %s", tx.Hash().String()))
	}
	instUsed[idx] = 1
	return true, nil
}

