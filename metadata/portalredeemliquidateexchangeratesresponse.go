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

type PortalRedeemLiquidateExchangeRatesResponse struct {
	MetadataBase
	RequestStatus    string
	ReqTxID          common.Hash
	RequesterAddrStr string
	RedeemAmount	 uint64
	Amount           uint64
	TokenID       	 string
}

func NewPortalRedeemLiquidateExchangeRatesResponse(
	requestStatus string,
	reqTxID common.Hash,
	requesterAddressStr string,
	redeemAmount uint64,
	amount uint64,
	tokenID string,
	metaType int,
) *PortalRedeemLiquidateExchangeRatesResponse {
	metadataBase := MetadataBase{
		Type: metaType,
	}
	return &PortalRedeemLiquidateExchangeRatesResponse{
		RequestStatus:    requestStatus,
		ReqTxID:          reqTxID,
		MetadataBase:     metadataBase,
		RequesterAddrStr: requesterAddressStr,
		RedeemAmount:     redeemAmount,
		Amount:           amount,
		TokenID:       	  tokenID,
	}
}

func (iRes PortalRedeemLiquidateExchangeRatesResponse) CheckTransactionFee(tr Transaction, minFee uint64, beaconHeight int64, db database.DatabaseInterface) bool {
	// no need to have fee for this tx
	return true
}

func (iRes PortalRedeemLiquidateExchangeRatesResponse) ValidateTxWithBlockChain(txr Transaction, bcr BlockchainRetriever, shardID byte, db database.DatabaseInterface) (bool, error) {
	// no need to validate tx with blockchain, just need to validate with requested tx (via RequestedTxID)
	return false, nil
}

func (iRes PortalRedeemLiquidateExchangeRatesResponse) ValidateSanityData(bcr BlockchainRetriever, txr Transaction, beaconHeight uint64) (bool, bool, error) {
	return false, true, nil
}

func (iRes PortalRedeemLiquidateExchangeRatesResponse) ValidateMetadataByItself() bool {
	// The validation just need to check at tx level, so returning true here
	return iRes.Type == PortalRedeemLiquidateExchangeRatesResponseMeta
}

func (iRes PortalRedeemLiquidateExchangeRatesResponse) Hash() *common.Hash {
	record := iRes.MetadataBase.Hash().String()
	record += iRes.RequestStatus
	record += iRes.ReqTxID.String()
	record += iRes.RequesterAddrStr
	record += strconv.FormatUint(iRes.RedeemAmount, 10)
	record += strconv.FormatUint(iRes.Amount, 10)
	record += iRes.TokenID
	// final hash
	hash := common.HashH([]byte(record))
	return &hash
}

func (iRes *PortalRedeemLiquidateExchangeRatesResponse) CalculateSize() uint64 {
	return calculateSize(iRes)
}

func (iRes PortalRedeemLiquidateExchangeRatesResponse) VerifyMinerCreatedTxBeforeGettingInBlock(
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
		if len(inst) < 4 { // this is not PortalRedeemRequest response instruction
			continue
		}
		instMetaType := inst[0]
		if instUsed[i] > 0 ||
			instMetaType != strconv.Itoa(PortalRedeemLiquidateExchangeRatesMeta) {
			continue
		}
		instReqStatus := inst[2]
		if instReqStatus != iRes.RequestStatus ||
			(instReqStatus != common.PortalRedeemLiquidateExchangeRatesSuccessChainStatus) {
			Logger.log.Error("WARNING - VALIDATION: status is not exactly, status  %v", instReqStatus)
			continue
		}

		var shardIDFromInst byte
		var txReqIDFromInst common.Hash
		var requesterAddrStrFromInst string
		var redeemAmountFromInst uint64
		var totalPTokenReceived uint64
		//var tokenIDStrFromInst string

		contentBytes := []byte(inst[3])
		var redeemReqContent PortalRedeemLiquidateExchangeRatesContent
		err := json.Unmarshal(contentBytes, &redeemReqContent)
		if err != nil {
			Logger.log.Error("WARNING - VALIDATION: an error occurred while parsing portal redeem liquidate exchange rates content: ", err)
			continue
		}

		shardIDFromInst = redeemReqContent.ShardID
		txReqIDFromInst = redeemReqContent.TxReqID
		requesterAddrStrFromInst = redeemReqContent.RedeemerIncAddressStr
		redeemAmountFromInst = redeemReqContent.RedeemAmount
		totalPTokenReceived = redeemReqContent.TotalPTokenReceived
		//tokenIDStrFromInst = redeemReqContent.TokenID

		if !bytes.Equal(iRes.ReqTxID[:], txReqIDFromInst[:]) ||
			shardID != shardIDFromInst {
			continue
		}

		if requesterAddrStrFromInst != iRes.RequesterAddrStr {
			Logger.log.Errorf("Error - VALIDATION: Requester address %v is not matching to Requester address in instruction %v", iRes.RequesterAddrStr, requesterAddrStrFromInst)
			continue
		}

		if totalPTokenReceived != iRes.Amount {
			Logger.log.Errorf("Error - VALIDATION:  totalPTokenReceived %v is not matching to  TotalPTokenReceived in instruction %v", iRes.Amount, redeemAmountFromInst)
			continue
		}

		if redeemAmountFromInst != iRes.RedeemAmount {
			Logger.log.Errorf("Error - VALIDATION: Redeem amount %v is not matching to redeem amount in instruction %v", iRes.RedeemAmount, redeemAmountFromInst)
			continue
		}

		key, err := wallet.Base58CheckDeserialize(requesterAddrStrFromInst)
		if err != nil {
			Logger.log.Info("WARNING - VALIDATION: an error occurred while deserializing requester address string: ", err)
			continue
		}

		PRVIDStr := common.PRVCoinID.String()
		_, pk, paidAmount, assetID := tx.GetTransferData()
		if !bytes.Equal(key.KeySet.PaymentAddress.Pk[:], pk[:]) ||
			redeemAmountFromInst != paidAmount ||
			PRVIDStr != assetID.String() {
			continue
		}
		idx = i
		break
	}

	if idx == -1 { // not found the issuance request tx for this response
		return false, fmt.Errorf(fmt.Sprintf("no PortalRedeemLiquidateExchangeRates instruction found for PortalRedeemLiquidateExchangeRatesResponse tx %s", tx.Hash().String()))
	}

	instUsed[idx] = 1
	return true, nil
}
