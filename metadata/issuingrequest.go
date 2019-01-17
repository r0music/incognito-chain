package metadata

import (
	"bytes"
	"errors"

	"github.com/ninjadotorg/constant/common"
	"github.com/ninjadotorg/constant/database"
	privacy "github.com/ninjadotorg/constant/privacy"
)

type IssuingRequest struct {
	ReceiverAddress privacy.PaymentAddress
	DepositedAmount uint64      // in US dollar
	AssetType       common.Hash // token id (one of types: Constant, BANK)
	// TODO: need an ID to verify with PrimeTrust
	MetadataBase
}

func NewIssuingRequest(
	receiverAddress privacy.PaymentAddress,
	depositedAmount uint64,
	assetType common.Hash,
	metaType int,
) *IssuingRequest {
	metadataBase := MetadataBase{
		Type: metaType,
	}
	issuingReq := &IssuingRequest{
		ReceiverAddress: receiverAddress,
		DepositedAmount: depositedAmount,
		AssetType:       assetType,
	}
	issuingReq.MetadataBase = metadataBase
	return issuingReq
}

func (iReq *IssuingRequest) ValidateTxWithBlockChain(
	txr Transaction,
	bcr BlockchainRetriever,
	chainID byte,
	db database.DatabaseInterface,
) (bool, error) {
	if bytes.Equal(iReq.AssetType[:], common.DCBTokenID[:]) {
		saleDBCTOkensByUSDData := bcr.GetDCBParams().SaleDCBTokensByUSDData
		height, err := bcr.GetTxChainHeight(txr)
		if height+1 > saleDBCTOkensByUSDData.EndBlock {
			return common.FalseValue, err
		}
		oracleParams := bcr.GetOracleParams()
		reqAmt := iReq.DepositedAmount / oracleParams.DCBToken
		if saleDBCTOkensByUSDData.Amount < reqAmt {
			return common.FalseValue, nil
		}
	}
	return common.TrueValue, nil
}

func (iReq *IssuingRequest) ValidateSanityData(bcr BlockchainRetriever, txr Transaction) (bool, bool, error) {
	if len(iReq.ReceiverAddress.Pk) == 0 {
		return common.FalseValue, common.FalseValue, errors.New("Wrong request info's receiver address")
	}
	if iReq.DepositedAmount == 0 {
		return common.FalseValue, common.FalseValue, errors.New("Wrong request info's deposited amount")
	}
	if iReq.Type == IssuingRequestMeta {
		return common.FalseValue, common.FalseValue, errors.New("Wrong request info's meta type")
	}
	if len(iReq.AssetType) != common.HashSize {
		return common.FalseValue, common.FalseValue, errors.New("Wrong request info's asset type")
	}
	return common.TrueValue, common.TrueValue, nil
}

func (iReq *IssuingRequest) ValidateMetadataByItself() bool {
	if iReq.Type != IssuingRequestMeta {
		return common.FalseValue
	}
	if !bytes.Equal(iReq.AssetType[:], common.DCBTokenID[:]) &&
		!bytes.Equal(iReq.AssetType[:], common.ConstantID[:]) {
		return common.FalseValue
	}
	return common.TrueValue
}

func (iReq *IssuingRequest) Hash() *common.Hash {
	record := iReq.ReceiverAddress.String()
	record += iReq.AssetType.String()
	record += string(iReq.DepositedAmount)
	record += iReq.MetadataBase.Hash().String()

	// final hash
	hash := common.DoubleHashH([]byte(record))
	return &hash
}
