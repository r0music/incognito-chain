package metadata

import (
	"github.com/ninjadotorg/constant/common"
	privacy "github.com/ninjadotorg/constant/privacy-protocol"
	"github.com/ninjadotorg/constant/transaction"
)

type BuySellRequest struct {
	PaymentAddress privacy.PaymentAddress
	AssetType      common.Hash // token id (note: for bond, this one is just bond token id prefix)
	Amount         uint64
	BuyPrice       uint64 // in Constant unit

	SaleID []byte // only when requesting to DCB

	MetadataBase
}

func NewBuySellRequest(bsReqData map[string]interface{}) *BuySellRequest {
	return &BuySellRequest{
		PaymentAddress: bsReqData["paymentAddress"].(privacy.PaymentAddress),
		AssetType:      bsReqData["assetType"].(common.Hash),
		Amount:         uint64(bsReqData["amount"].(float64)),
		BuyPrice:       uint64(bsReqData["buyPrice"].(float64)),
		SaleID:         bsReqData["saleId"].([]byte),
	}
}

func (bsReq *BuySellRequest) Validate() error {
	return nil
}

func (bsReq *BuySellRequest) Process() error {
	return nil
}

func (bsReq *BuySellRequest) CheckTransactionFee(tr transaction.TxRetriever, minFee uint64) bool {
	txFee := tr.GetTxFee()
	if txFee < minFee {
		return false
	}
	return true
}

func (bsReq *BuySellRequest) ValidateTxWithBlockChain(bcr transaction.BlockchainRetriever, chainID byte) (bool, error) {
	return true, nil
}
