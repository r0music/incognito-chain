package rpcserver

import (
	"fmt"
	"github.com/incognitochain/incognito-chain/rpcserver/rpcservice"
	"sort"

	"github.com/incognitochain/incognito-chain/blockchain"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/transaction"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/pkg/errors"
)

func (rpcServer HttpServer) chooseOutsCoinByKeyset(paymentInfos []*privacy.PaymentInfo,
	estimateFeeCoinPerKb int64, numBlock uint64, keyset *incognitokey.KeySet, shardIDSender byte,
	hasPrivacy bool,
	metadataParam metadata.Metadata,
	customTokenParams *transaction.CustomTokenParamTx,
	privacyCustomTokenParams *transaction.CustomTokenPrivacyParamTx,
) ([]*privacy.InputCoin, uint64, *rpcservice.RPCError) {
	// estimate fee according to 8 recent block
	if numBlock == 0 {
		numBlock = 1000
	}
	// calculate total amount to send
	totalAmmount := uint64(0)
	for _, receiver := range paymentInfos {
		totalAmmount += receiver.Amount
	}

	// get list outputcoins tx
	prvCoinID := &common.Hash{}
	prvCoinID.SetBytes(common.PRVCoinID[:])
	outCoins, err := rpcServer.config.BlockChain.GetListOutputCoinsByKeyset(keyset, shardIDSender, prvCoinID)
	if err != nil {
		return nil, 0, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err)
	}
	// remove out coin in mem pool
	outCoins, err = rpcServer.txMemPoolService.FilterMemPoolOutcoinsToSpent(outCoins)
	if err != nil {
		return nil, 0, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err)
	}
	if len(outCoins) == 0 && totalAmmount > 0 {
		return nil, 0, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, errors.New("not enough output coin"))
	}
	// Use Knapsack to get candiate output coin
	candidateOutputCoins, outCoins, candidateOutputCoinAmount, err := rpcServer.chooseBestOutCoinsToSpent(outCoins, totalAmmount)
	if err != nil {
		return nil, 0, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err)
	}
	// refund out put for sender
	overBalanceAmount := candidateOutputCoinAmount - totalAmmount
	if overBalanceAmount > 0 {
		// add more into output for estimate fee
		paymentInfos = append(paymentInfos, &privacy.PaymentInfo{
			PaymentAddress: keyset.PaymentAddress,
			Amount:         overBalanceAmount,
		})
	}

	// check real fee(nano PRV) per tx
	realFee, _, _ := rpcServer.estimateFee(estimateFeeCoinPerKb, candidateOutputCoins,
		paymentInfos, shardIDSender, numBlock, hasPrivacy,
		metadataParam, customTokenParams,
		privacyCustomTokenParams)

	if totalAmmount == 0 && realFee == 0 {
		if metadataParam != nil {
			metadataType := metadataParam.GetType()
			switch metadataType {
			case metadata.WithDrawRewardRequestMeta:
				{
					return nil, realFee, nil
				}
			}
			return nil, realFee, rpcservice.NewRPCError(rpcservice.RejectInvalidFeeError, errors.New(fmt.Sprintf("totalAmmount: %+v, realFee: %+v", totalAmmount, realFee)))
		}
		if privacyCustomTokenParams != nil {
			// for privacy token
			return nil, 0, nil
		}
	}

	needToPayFee := int64((totalAmmount + realFee) - candidateOutputCoinAmount)
	// if not enough to pay fee
	if needToPayFee > 0 {
		if len(outCoins) > 0 {
			candidateOutputCoinsForFee, _, _, err1 := rpcServer.chooseBestOutCoinsToSpent(outCoins, uint64(needToPayFee))
			if err != nil {
				return nil, 0, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err1)
			}
			candidateOutputCoins = append(candidateOutputCoins, candidateOutputCoinsForFee...)
		}
	}
	// convert to inputcoins
	inputCoins := transaction.ConvertOutputCoinToInputCoin(candidateOutputCoins)
	return inputCoins, realFee, nil
}

func (rpcServer HttpServer) buildRawTransaction(params interface{}, meta metadata.Metadata) (*transaction.Tx, *rpcservice.RPCError) {
	Logger.log.Infof("Params: \n%+v\n\n\n", params)

	/******* START Fetch all component to ******/
	// all component
	arrayParams := common.InterfaceSlice(params)

	// param #1: private key of sender
	senderKeyParam := arrayParams[0]
	senderKeySet, shardIDSender, err := rpcservice.GetKeySetFromPrivateKeyParams(senderKeyParam.(string))
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.InvalidSenderPrivateKeyError, err)
	}

	// param #2: list receiver
	receiversPaymentAddressStrParam := make(map[string]interface{})
	if arrayParams[1] != nil {
		receiversPaymentAddressStrParam = arrayParams[1].(map[string]interface{})
	}
	paymentInfos := make([]*privacy.PaymentInfo, 0)
	for paymentAddressStr, amount := range receiversPaymentAddressStrParam {
		keyWalletReceiver, err := wallet.Base58CheckDeserialize(paymentAddressStr)
		if err != nil {
			return nil, rpcservice.NewRPCError(rpcservice.InvalidReceiverPaymentAddressError, err)
		}
		paymentInfo := &privacy.PaymentInfo{
			Amount:         uint64(amount.(float64)),
			PaymentAddress: keyWalletReceiver.KeySet.PaymentAddress,
		}
		paymentInfos = append(paymentInfos, paymentInfo)
	}

	// param #3: estimation fee nano P per kb
	estimateFeeCoinPerKb := int64(arrayParams[2].(float64))

	// param #4: hasPrivacyCoin flag: 1 or -1
	hasPrivacyCoin := int(arrayParams[3].(float64)) > 0
	/********* END Fetch all component to *******/

	// param #4 for metadata

	// param#6: info (option)
	info := []byte{}
	if len(arrayParams) > 5 {
		infoStr := arrayParams[5].(string)
		info = []byte(infoStr)
	}

	/******* START choose output native coins(PRV), which is used to create tx *****/
	inputCoins, realFee, err1 := rpcServer.chooseOutsCoinByKeyset(paymentInfos, estimateFeeCoinPerKb, 0, senderKeySet, shardIDSender, hasPrivacyCoin, meta, nil, nil)
	if err1 != nil {
		return nil, err1
	}

	/******* END GET output coins native coins(PRV), which is used to create tx *****/

	// START create tx
	// missing flag for privacy
	// false by default
	//fmt.Printf("#inputCoins: %d\n", len(inputCoins))
	tx := transaction.Tx{}
	err = tx.Init(
		transaction.NewTxPrivacyInitParams(&senderKeySet.PrivateKey,
			paymentInfos,
			inputCoins,
			realFee,
			hasPrivacyCoin,
			*rpcServer.config.Database,
			nil, // use for prv coin -> nil is valid
			meta,
			info,
		))
	// END create tx

	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.CreateTxDataError, err)
	}

	return &tx, nil
}

func (rpcServer HttpServer) buildTokenParam(tokenParamsRaw map[string]interface{}, senderKeySet *incognitokey.KeySet, shardIDSender byte)(
	*transaction.CustomTokenParamTx, *transaction.CustomTokenPrivacyParamTx, *rpcservice.RPCError) {

	var customTokenParams *transaction.CustomTokenParamTx
	var customPrivacyTokenParam *transaction.CustomTokenPrivacyParamTx
	var err *rpcservice.RPCError

	isPrivacy := tokenParamsRaw["Privacy"].(bool)
	if !isPrivacy {
		// Check normal custom token param
		customTokenParams, _, err = rpcServer.buildCustomTokenParam(tokenParamsRaw, senderKeySet)
		if err != nil {
			return nil, nil, err
		}
	} else {
		// Check privacy custom token param
		customPrivacyTokenParam, _, _, err = rpcServer.buildPrivacyCustomTokenParam(tokenParamsRaw, senderKeySet, shardIDSender)
		if err != nil {
			return nil, nil, err
		}
	}

	return customTokenParams, customPrivacyTokenParam, nil

}

func (rpcServer HttpServer) buildCustomTokenParam(tokenParamsRaw map[string]interface{}, senderKeySet *incognitokey.KeySet) (*transaction.CustomTokenParamTx, map[common.Hash]transaction.TxNormalToken, *rpcservice.RPCError) {
	tokenParams := &transaction.CustomTokenParamTx{
		PropertyID:     tokenParamsRaw["TokenID"].(string),
		PropertyName:   tokenParamsRaw["TokenName"].(string),
		PropertySymbol: tokenParamsRaw["TokenSymbol"].(string),
		TokenTxType:    int(tokenParamsRaw["TokenTxType"].(float64)),
		Amount:         uint64(tokenParamsRaw["TokenAmount"].(float64)),
	}
	voutsAmount := int64(0)
	tokenParams.Receiver, voutsAmount, _ = transaction.CreateCustomTokenReceiverArray(tokenParamsRaw["TokenReceivers"])
	switch tokenParams.TokenTxType {
	case transaction.CustomTokenTransfer:
		{
			tokenID, err := common.Hash{}.NewHashFromStr(tokenParams.PropertyID)
			if err != nil {
				return nil, nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.Wrap(err, "Token ID is invalid"))
			}

			//if _, ok := listCustomTokens[*tokenID]; !ok {
			//	return nil, nil, NewRPCError(ErrRPCInvalidParams, errors.New("Invalid Token ID"))
			//}

			existed := rpcServer.config.BlockChain.CustomTokenIDExisted(tokenID)
			if !existed {
				return nil, nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Invalid Token ID"))
			}

			unspentTxTokenOuts, err := rpcServer.config.BlockChain.GetUnspentTxCustomTokenVout(*senderKeySet, tokenID)
			Logger.log.Info("buildRawCustomTokenTransaction ", unspentTxTokenOuts)
			if err != nil {
				return nil, nil, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, errors.New("Token out invalid"))
			}
			if len(unspentTxTokenOuts) == 0 {
				return nil, nil, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, errors.New("Token out invalid"))
			}
			txTokenIns := []transaction.TxTokenVin{}
			txTokenInsAmount := uint64(0)
			for _, out := range unspentTxTokenOuts {
				item := transaction.TxTokenVin{
					PaymentAddress:  out.PaymentAddress,
					TxCustomTokenID: out.GetTxCustomTokenID(),
					VoutIndex:       out.GetIndex(),
				}
				// create signature by keyset -> base58check.encode of txtokenout double hash
				signature, err := senderKeySet.Sign(out.Hash()[:])
				if err != nil {
					return nil, nil, rpcservice.NewRPCError(rpcservice.CanNotSignError, err)
				}
				// add signature to TxTokenVin to use token utxo
				item.Signature = base58.Base58Check{}.Encode(signature, 0)
				txTokenIns = append(txTokenIns, item)
				txTokenInsAmount += out.Value
				voutsAmount -= int64(out.Value)
				if voutsAmount <= 0 {
					break
				}
			}
			tokenParams.SetVins(txTokenIns)
			tokenParams.SetVinsAmount(txTokenInsAmount)
		}
	case transaction.CustomTokenInit:
		{
			if tokenParams.Receiver[0].Value != tokenParams.Amount { // Init with wrong max amount of custom token
				return nil, nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Init with wrong max amount of property"))
			}
		}
	}
	//return tokenParams, listCustomTokens, nil
	return tokenParams, nil, nil
}

// buildRawCustomTokenTransaction ...
func (rpcServer HttpServer) buildRawCustomTokenTransaction(
	params interface{},
	metaData metadata.Metadata,
) (*transaction.TxNormalToken, *rpcservice.RPCError) {
	// all params
	arrayParams := common.InterfaceSlice(params)

	// param #1: private key of sender
	senderKeyParam := arrayParams[0]
	var err error
	senderKeySet, shardIDSender, err := rpcservice.GetKeySetFromPrivateKeyParams(senderKeyParam.(string))
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.GetKeySetFromPrivateKeyError, err)
	}

	// param #2: list receiver
	receiversPaymentAddressParam := make(map[string]interface{})
	if arrayParams[1] != nil {
		receiversPaymentAddressParam = arrayParams[1].(map[string]interface{})
	}
	paymentInfos := make([]*privacy.PaymentInfo, 0)
	for paymentAddressStr, amount := range receiversPaymentAddressParam {
		keyWalletReceiver, err := wallet.Base58CheckDeserialize(paymentAddressStr)
		if err != nil {
			return nil, rpcservice.NewRPCError(rpcservice.InvalidReceiverPaymentAddressError, err)
		}
		paymentInfo := &privacy.PaymentInfo{
			Amount:         uint64(amount.(float64)),
			PaymentAddress: keyWalletReceiver.KeySet.PaymentAddress,
		}
		paymentInfos = append(paymentInfos, paymentInfo)
	}

	// param #3: estimation fee coin per kb
	estimateFeeCoinPerKb := int64(arrayParams[2].(float64))

	// param #4: hasPrivacyCoin flag
	hasPrivacyCoin := int(arrayParams[3].(float64)) > 0

	// param #5: token params
	tokenParamsRaw := arrayParams[4].(map[string]interface{})
	tokenParams, listCustomTokens, err := rpcServer.buildCustomTokenParam(tokenParamsRaw, senderKeySet)
	_ = listCustomTokens
	if err.(*rpcservice.RPCError) != nil {
		return nil, err.(*rpcservice.RPCError)
	}
	/******* START choose output coins native coins(PRV), which is used to create tx *****/
	inputCoins, realFee, err := rpcServer.chooseOutsCoinByKeyset(paymentInfos, estimateFeeCoinPerKb, 0,
		senderKeySet, shardIDSender, hasPrivacyCoin,
		metaData, tokenParams, nil)
	if err.(*rpcservice.RPCError) != nil {
		return nil, err.(*rpcservice.RPCError)
	}
	if len(paymentInfos) == 0 && realFee == 0 {
		hasPrivacyCoin = false
	}
	/******* END GET output coins native coins(PRV), which is used to create tx *****/

	tx := &transaction.TxNormalToken{}
	err = tx.Init(
		transaction.NewTxNormalTokenInitParam(&senderKeySet.PrivateKey,
			nil,
			inputCoins,
			realFee,
			tokenParams,
			//listCustomTokens,
			*rpcServer.config.Database,
			metaData,
			hasPrivacyCoin,
			shardIDSender))
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.CreateTxDataError, err)
	}

	return tx, nil
}

func (rpcServer HttpServer) buildPrivacyCustomTokenParam(tokenParamsRaw map[string]interface{}, senderKeySet *incognitokey.KeySet, shardIDSender byte) (*transaction.CustomTokenPrivacyParamTx, map[common.Hash]transaction.TxCustomTokenPrivacy, map[common.Hash]blockchain.CrossShardTokenPrivacyMetaData, *rpcservice.RPCError) {
	tokenParams := &transaction.CustomTokenPrivacyParamTx{
		PropertyID:     tokenParamsRaw["TokenID"].(string),
		PropertyName:   tokenParamsRaw["TokenName"].(string),
		PropertySymbol: tokenParamsRaw["TokenSymbol"].(string),
		TokenTxType:    int(tokenParamsRaw["TokenTxType"].(float64)),
		Amount:         uint64(tokenParamsRaw["TokenAmount"].(float64)),
		TokenInput:     nil,
		Fee:            uint64(tokenParamsRaw["TokenFee"].(float64)),
	}
	voutsAmount := int64(0)
	tokenParams.Receiver, voutsAmount = transaction.CreateCustomTokenPrivacyReceiverArray(tokenParamsRaw["TokenReceivers"])

	// get list custom token
	switch tokenParams.TokenTxType {
	case transaction.CustomTokenTransfer:
		{
			tokenID, err := common.Hash{}.NewHashFromStr(tokenParams.PropertyID)
			if err != nil {
				return nil, nil, nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, err)
			}
			existed := rpcServer.config.BlockChain.PrivacyCustomTokenIDExisted(tokenID)
			existedCrossShard := rpcServer.config.BlockChain.PrivacyCustomTokenIDCrossShardExisted(tokenID)
			if !existed && !existedCrossShard {
				return nil, nil, nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Invalid Token ID"))
			}
			outputTokens, err := rpcServer.config.BlockChain.GetListOutputCoinsByKeyset(senderKeySet, shardIDSender, tokenID)
			if err != nil {
				return nil, nil, nil, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err)
			}
			outputTokens, err = rpcServer.txMemPoolService.FilterMemPoolOutcoinsToSpent(outputTokens)
			if err != nil {
				return nil, nil, nil, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err)
			}
			candidateOutputTokens, _, _, err := rpcServer.chooseBestOutCoinsToSpent(outputTokens, uint64(voutsAmount))
			if err != nil {
				return nil, nil, nil, rpcservice.NewRPCError(rpcservice.GetOutputCoinError, err)
			}
			intputToken := transaction.ConvertOutputCoinToInputCoin(candidateOutputTokens)
			tokenParams.TokenInput = intputToken
		}
	case transaction.CustomTokenInit:
		{
			if tokenParams.Receiver[0].Amount != tokenParams.Amount { // Init with wrong max amount of custom token
				return nil, nil, nil, rpcservice.NewRPCError(rpcservice.RPCInvalidParamsError, errors.New("Init with wrong max amount of property"))
			}
		}
	}
	return tokenParams, nil, nil, nil
}

// buildRawCustomTokenTransaction ...
func (rpcServer HttpServer) buildRawPrivacyCustomTokenTransaction(
	params interface{},
	metaData metadata.Metadata,
) (*transaction.TxCustomTokenPrivacy, *rpcservice.RPCError) {
	// all component
	arrayParams := common.InterfaceSlice(params)

	/****** START FEtch data from component *********/
	// param #1: private key of sender
	senderKeyParam := arrayParams[0]
	senderKeySet, shardIDSender, err := rpcservice.GetKeySetFromPrivateKeyParams(senderKeyParam.(string))
	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.InvalidSenderPrivateKeyError, err)
	}

	// param #2: list receiver
	receiversPaymentAddressStrParam := make(map[string]interface{})
	if arrayParams[1] != nil {
		receiversPaymentAddressStrParam = arrayParams[1].(map[string]interface{})
	}
	paymentInfos := make([]*privacy.PaymentInfo, 0)
	for paymentAddressStr, amount := range receiversPaymentAddressStrParam {
		keyWalletReceiver, err := wallet.Base58CheckDeserialize(paymentAddressStr)
		if err != nil {
			return nil, rpcservice.NewRPCError(rpcservice.InvalidReceiverPaymentAddressError, err)
		}
		paymentInfo := &privacy.PaymentInfo{
			Amount:         uint64(amount.(float64)),
			PaymentAddress: keyWalletReceiver.KeySet.PaymentAddress,
		}
		paymentInfos = append(paymentInfos, paymentInfo)
	}

	// param #3: estimation fee coin per kb
	estimateFeeCoinPerKb := int64(arrayParams[2].(float64))

	// param #4: hasPrivacy flag for native coin
	hasPrivacyCoin := int(arrayParams[3].(float64)) > 0

	// param #5: token component
	tokenParamsRaw := arrayParams[4].(map[string]interface{})
	tokenParams, listCustomTokens, listCustomTokenCrossShard, err := rpcServer.buildPrivacyCustomTokenParam(tokenParamsRaw, senderKeySet, shardIDSender)

	_ = listCustomTokenCrossShard
	_ = listCustomTokens
	if err.(*rpcservice.RPCError) != nil {
		return nil, err.(*rpcservice.RPCError)
	}

	// param #6: hasPrivacyToken flag for token
	hasPrivacyToken := true
	if len(arrayParams) >= 6 {
		hasPrivacyToken = int(arrayParams[5].(float64)) > 0
	}

	// param#7: info (option)
	info := []byte{}
	if len(arrayParams) >= 7 {
		infoStr := arrayParams[6].(string)
		info = []byte(infoStr)
	}

	/****** END FEtch data from params *********/

	/******* START choose output native coins(PRV), which is used to create tx *****/
	var inputCoins []*privacy.InputCoin
	var realFeePrv uint64
	inputCoins, realFeePrv, err = rpcServer.chooseOutsCoinByKeyset(paymentInfos,
		estimateFeeCoinPerKb, 0, senderKeySet,
		shardIDSender, hasPrivacyCoin, nil,
		nil, tokenParams)
	if err.(*rpcservice.RPCError) != nil {
		return nil, err.(*rpcservice.RPCError)
	}
	if len(paymentInfos) == 0 && realFeePrv == 0 {
		hasPrivacyCoin = false
	}
	/******* END GET output coins native coins(PRV), which is used to create tx *****/

	tx := &transaction.TxCustomTokenPrivacy{}
	err = tx.Init(
		transaction.NewTxPrivacyTokenInitParams(&senderKeySet.PrivateKey,
			nil,
			inputCoins,
			realFeePrv,
			tokenParams,
			*rpcServer.config.Database,
			metaData,
			hasPrivacyCoin,
			hasPrivacyToken,
			shardIDSender, info))

	if err != nil {
		return nil, rpcservice.NewRPCError(rpcservice.CreateTxDataError, err)
	}

	return tx, nil
}

// estimateFeeWithEstimator - only estimate fee by estimator and return fee per kb
func (rpcServer HttpServer) estimateFeeWithEstimator(defaultFee int64, shardID byte, numBlock uint64, tokenId *common.Hash) uint64 {
	estimateFeeCoinPerKb := uint64(0)
	if defaultFee == -1 {
		if _, ok := rpcServer.config.FeeEstimator[shardID]; ok {
			temp, _ := rpcServer.config.FeeEstimator[shardID].EstimateFee(numBlock, tokenId)
			estimateFeeCoinPerKb = uint64(temp)
		}
		if estimateFeeCoinPerKb == 0 {
			if feeEstimator, ok := rpcServer.config.FeeEstimator[shardID]; ok {
				estimateFeeCoinPerKb = feeEstimator.GetLimitFee()
			}
		}
	} else {
		estimateFeeCoinPerKb = uint64(defaultFee)
	}
	return estimateFeeCoinPerKb
}

// estimateFee - estimate fee from tx data and return real full fee, fee per kb and real tx size
func (rpcServer HttpServer) estimateFee(
	defaultFee int64,
	candidateOutputCoins []*privacy.OutputCoin,
	paymentInfos []*privacy.PaymentInfo, shardID byte,
	numBlock uint64, hasPrivacy bool,
	metadata metadata.Metadata,
	customTokenParams *transaction.CustomTokenParamTx,
	privacyCustomTokenParams *transaction.CustomTokenPrivacyParamTx) (uint64, uint64, uint64) {
	if numBlock == 0 {
		numBlock = 1000
	}
	// check real fee(nano PRV) per tx
	var realFee uint64
	estimateFeeCoinPerKb := uint64(0)
	estimateTxSizeInKb := uint64(0)

	tokenId := &common.Hash{}
	if privacyCustomTokenParams != nil {
		tokenId, _ = common.Hash{}.NewHashFromStr(privacyCustomTokenParams.PropertyID)
	}

	estimateFeeCoinPerKb = rpcServer.estimateFeeWithEstimator(defaultFee, shardID, numBlock, tokenId)

	if rpcServer.config.Wallet != nil {
		estimateFeeCoinPerKb += uint64(rpcServer.config.Wallet.GetConfig().IncrementalFee)
	}

	limitFee := uint64(0)
	if feeEstimator, ok := rpcServer.config.FeeEstimator[shardID]; ok {
		limitFee = feeEstimator.GetLimitFee()
	}
	estimateTxSizeInKb = transaction.EstimateTxSize(transaction.NewEstimateTxSizeParam(candidateOutputCoins, paymentInfos, hasPrivacy, metadata, customTokenParams, privacyCustomTokenParams, limitFee))

	realFee = uint64(estimateFeeCoinPerKb) * uint64(estimateTxSizeInKb)
	return realFee, estimateFeeCoinPerKb, estimateTxSizeInKb
}

// chooseBestOutCoinsToSpent returns list of unspent coins for spending with amount
func (rpcServer HttpServer) chooseBestOutCoinsToSpent(outCoins []*privacy.OutputCoin, amount uint64) (resultOutputCoins []*privacy.OutputCoin, remainOutputCoins []*privacy.OutputCoin, totalResultOutputCoinAmount uint64, err error) {
	resultOutputCoins = make([]*privacy.OutputCoin, 0)
	remainOutputCoins = make([]*privacy.OutputCoin, 0)
	totalResultOutputCoinAmount = uint64(0)

	// either take the smallest coins, or a single largest one
	var outCoinOverLimit *privacy.OutputCoin
	outCoinsUnderLimit := make([]*privacy.OutputCoin, 0)

	for _, outCoin := range outCoins {
		if outCoin.CoinDetails.GetValue() < amount {
			outCoinsUnderLimit = append(outCoinsUnderLimit, outCoin)
		} else if outCoinOverLimit == nil {
			outCoinOverLimit = outCoin
		} else if outCoinOverLimit.CoinDetails.GetValue() > outCoin.CoinDetails.GetValue() {
			remainOutputCoins = append(remainOutputCoins, outCoin)
		} else {
			remainOutputCoins = append(remainOutputCoins, outCoinOverLimit)
			outCoinOverLimit = outCoin
		}
	}

	sort.Slice(outCoinsUnderLimit, func(i, j int) bool {
		return outCoinsUnderLimit[i].CoinDetails.GetValue() < outCoinsUnderLimit[j].CoinDetails.GetValue()
	})

	for _, outCoin := range outCoinsUnderLimit {
		if totalResultOutputCoinAmount < amount {
			totalResultOutputCoinAmount += outCoin.CoinDetails.GetValue()
			resultOutputCoins = append(resultOutputCoins, outCoin)
		} else {
			remainOutputCoins = append(remainOutputCoins, outCoin)
		}
	}

	if outCoinOverLimit != nil && (outCoinOverLimit.CoinDetails.GetValue() > 2*amount || totalResultOutputCoinAmount < amount) {
		remainOutputCoins = append(remainOutputCoins, resultOutputCoins...)
		resultOutputCoins = []*privacy.OutputCoin{outCoinOverLimit}
		totalResultOutputCoinAmount = outCoinOverLimit.CoinDetails.GetValue()
	} else if outCoinOverLimit != nil {
		remainOutputCoins = append(remainOutputCoins, outCoinOverLimit)
	}

	if totalResultOutputCoinAmount < amount {
		return resultOutputCoins, remainOutputCoins, totalResultOutputCoinAmount, errors.New("Not enough coin")
	} else {
		return resultOutputCoins, remainOutputCoins, totalResultOutputCoinAmount, nil
	}
}

// GetPaymentAddressFromPrivateKeyParams- deserialize a private key string
// and return paymentaddress object which relate to private key exactly
func (rpcServer HttpServer) GetPaymentAddressFromPrivateKeyParams(senderKeyParam string) (*privacy.PaymentAddress, error) {
	keySet, _, err := rpcservice.GetKeySetFromPrivateKeyParams(senderKeyParam)
	if err != nil {
		return nil, err
	}
	return &keySet.PaymentAddress, err
}

