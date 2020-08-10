package rpcserver

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/dataaccessobject/rawdbv2"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/rpcserver/bean"
	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
	"github.com/incognitochain/incognito-chain/rpcserver/rpcservice"
	"github.com/incognitochain/incognito-chain/wallet"
)

// handleUnstakeTx - RPC create and send unstake tx to network
func (httpServer *HttpServer) handleUnstakeTx(params interface{}, closeChan <-chan struct{}) (interface{}, *rpcservice.RPCError) {
	var err error
	data, err := httpServer.handleUnstakeRawTx(params)
	if err.(*rpcservice.RPCError) != nil {
		return nil, rpcservice.NewRPCError(rpcservice.CreateTxDataError, err)
	}
	tx := data.(jsonresult.CreateTransactionResult)
	base58CheckData := tx.Base58CheckData

	newParam := make([]interface{}, 0)
	newParam = append(newParam, base58CheckData)
	Logger.log.Info("[unstake] tx:", tx)
	sendResult, err := httpServer.handleSendRawTransaction(newParam, closeChan)
	if err.(*rpcservice.RPCError) != nil {
		Logger.log.Info("[unstake] err.(*rpcservice.RPCError):", err.(*rpcservice.RPCError))
		Logger.log.Info("[unstake] err:", err)
		return nil, rpcservice.NewRPCError(rpcservice.SendTxDataError, err)
	}
	result := jsonresult.NewCreateTransactionResult(nil, sendResult.(jsonresult.CreateTransactionResult).TxID, nil, tx.ShardID)
	return result, nil
}

// handleUnstakeRawTx - handle unstake tx request
// and return raw data for making raw transaction
func (httpServer *HttpServer) handleUnstakeRawTx(params interface{}) (interface{}, *rpcservice.RPCError) {

	paramsArray := common.InterfaceSlice(params)
	if paramsArray == nil || len(paramsArray) < 5 {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("param must be an array at least 5 element"))
	}

	createRawTxParam, errNewParam := bean.NewCreateRawTxParam(params)
	if errNewParam != nil {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errNewParam)
	}

	keyWallet := new(wallet.KeyWallet)
	keyWallet.KeySet = *createRawTxParam.SenderKeySet
	funderPaymentAddress := keyWallet.Base58CheckSerialize(wallet.PaymentAddressType)
	_ = funderPaymentAddress

	//Get data to create meta data
	data, ok := paramsArray[4].(map[string]interface{})
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, fmt.Errorf("Invalid unstaking metadata %+v", paramsArray[4]))
	}

	//Get staking type
	unStakingType, ok := data["UnStakingType"].(float64)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, fmt.Errorf("Invalid UnStaking Type For Staking Transaction %+v", data["UnStakingType"]))
	}

	//Get Candidate Payment Address
	candidatePaymentAddress, ok := data["CandidatePaymentAddress"].(string)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, fmt.Errorf("Invalid Producer Payment Address for Staking Transaction %+v", data["CandidatePaymentAddress"]))
	}
	// Get private seed, a.k.a mining key
	privateSeed, ok := data["PrivateSeed"].(string)
	if !ok {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, fmt.Errorf("Invalid Private Seed for Staking Transaction %+v", data["PrivateSeed"]))
	}
	privateSeedBytes, ver, err := base58.Base58Check{}.Decode(privateSeed)
	if (err != nil) || (ver != common.ZeroByte) {
		return nil, rpcservice.NewRPCError(rpcservice.UnexpectedError, errors.New("Decode privateseed failed"))
	}

	// Get candidate publickey
	candidateWallet, err := wallet.Base58CheckDeserialize(candidatePaymentAddress)
	if err != nil || candidateWallet == nil {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Base58CheckDeserialize candidate Payment Address failed"))
	}
	pk := candidateWallet.KeySet.PaymentAddress.Pk

	committeePK, err := incognitokey.NewCommitteeKeyFromSeed(privateSeedBytes, pk)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, err)
	}

	committeePKBytes, err := committeePK.Bytes()
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, err)
	}

	unStakingMetadata, err := metadata.NewUnStakingMetadata(int(unStakingType), base58.Base58Check{}.Encode(committeePKBytes, common.ZeroByte))
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, err)
	}
	txID, txBytes, txShardID, err := httpServer.txService.CreateRawTransaction(createRawTxParam, unStakingMetadata)
	if err.(*rpcservice.RPCError) != nil {
		return nil, rpcservice.NewRPCError(rpcservice.CreateTxDataError, err)
	}

	allViews := []*blockchain.BeaconBestState{}
	beaconDB := httpServer.blockService.BlockChain.GetBeaconChainDatabase()
	beaconViews, err := rawdbv2.GetBeaconViews(beaconDB)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.GetAllBeaconViews, err)
	}

	err = json.Unmarshal(beaconViews, &allViews)
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.GetAllBeaconViews, err)
	}

	for _, v := range allViews {
		publickey, err := committeePK.ToBase58()
		if err != nil {
			return nil, rpcservice.NewRPCError(rpcservice.GetAllBeaconViews, err)
		}
		if v.Unstake()[publickey] {
			return nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("User has already unstaked"))
		}
	}

	result := jsonresult.CreateTransactionResult{
		TxID:            txID.String(),
		Base58CheckData: base58.Base58Check{}.Encode(txBytes, common.ZeroByte),
		ShardID:         txShardID,
	}

	return result, nil
}
