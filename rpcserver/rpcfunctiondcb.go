package rpcserver

import (
	"encoding/hex"

	"github.com/constant-money/constant-chain/common"
	"github.com/constant-money/constant-chain/metadata"
	"github.com/constant-money/constant-chain/privacy"
	"github.com/constant-money/constant-chain/wallet"
)

// handleGetDCBParams - get dcb component
func (rpcServer RpcServer) handleGetDCBParams(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	constitution := rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBConstitution
	dcbParam := constitution.DCBParams
	results := make(map[string]interface{})
	results["ConstitutionIndex"] = constitution.ConstitutionIndex
	results["DCBParams"] = dcbParam
	results["ExecuteDuration"] = constitution.ExecuteDuration
	results["Explanation"] = constitution.Explanation
	return results, nil
}

// handleGetDCBConstitution - get dcb constitution
func (rpcServer RpcServer) handleGetDCBConstitution(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	constitution := rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBConstitution
	return constitution, nil
}

// handleGetListDCBBoard - return list payment address of DCB board
func (rpcServer RpcServer) handleGetListDCBBoard(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	res := ListPaymentAddressToListString(rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBGovernor.BoardPaymentAddress)
	return res, nil
}

func (rpcServer RpcServer) handleGetListDCBBoardPayment(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	res := []string{}
	listPayment := rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBGovernor.BoardPaymentAddress
	for _, i := range listPayment {
		wtf := wallet.KeyWallet{}
		wtf.KeySet.PaymentAddress = i
		res = append(res, wtf.Base58CheckSerialize(wallet.PaymentAddressType))
	}
	return res, nil
}

func (rpcServer RpcServer) handleAppendListDCBBoard(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	arrayParams := common.InterfaceSlice(params)
	senderKey := arrayParams[0].(string)
	paymentAddress, _ := metadata.GetPaymentAddressFromSenderKeyParams(senderKey)
	rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBGovernor.BoardPaymentAddress =
		append(rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBGovernor.BoardPaymentAddress, *paymentAddress)
	res := ListPaymentAddressToListString(rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.DCBGovernor.BoardPaymentAddress)
	return res, nil
}

func ListPaymentAddressToListString(addresses []privacy.PaymentAddress) []string {
	res := make([]string, 0)
	for _, i := range addresses {
		pk := hex.EncodeToString(i.Pk)
		res = append(res, pk)
	}
	return res
}

func (rpcServer RpcServer) handleGetConstantCirculating(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	type result struct {
		Total uint64
	}
	return result{Total: uint64(0)}, nil
}

// handleGetBankFund returns bank fund stored on Beacon chain
func (rpcServer RpcServer) handleGetBankFund(params interface{}, closeChan <-chan struct{}) (interface{}, *RPCError) {
	bankFund := rpcServer.config.BlockChain.BestState.Beacon.StabilityInfo.BankFund
	return bankFund, nil
}
