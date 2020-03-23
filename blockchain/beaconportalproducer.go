package blockchain

import (
	"encoding/base64"
	"encoding/json"
	"github.com/binance-chain/go-sdk/types/msg"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database/lvdb"
	"github.com/incognitochain/incognito-chain/metadata"
	"github.com/incognitochain/incognito-chain/relaying/bnb"
	"strconv"
)

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildCustodianDepositInst(
	custodianAddressStr string,
	depositedAmount uint64,
	remoteAddresses []lvdb.RemoteAddress,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	custodianDepositContent := metadata.PortalCustodianDepositContent{
		IncogAddressStr: custodianAddressStr,
		RemoteAddresses: remoteAddresses,
		DepositedAmount: depositedAmount,
		TxReqID:         txReqID,
		ShardID:         shardID,
	}
	custodianDepositContentBytes, _ := json.Marshal(custodianDepositContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(custodianDepositContentBytes),
	}
}

func buildRequestPortingInst(
	metaType int,
	shardID byte,
	reqStatus string,
	uniqueRegisterId string,
	incogAddressStr string,
	pTokenId string,
	registerAmount uint64,
	portingFee uint64,
	custodian []*lvdb.MatchingPortingCustodianDetail,
	txReqID common.Hash,
) []string {
	portingRequestContent := metadata.PortalPortingRequestContent{
		UniqueRegisterId: uniqueRegisterId,
		IncogAddressStr:  incogAddressStr,
		PTokenId:         pTokenId,
		RegisterAmount:   registerAmount,
		PortingFee:       portingFee,
		Custodian:        custodian,
		TxReqID:          txReqID,
	}

	portingRequestContentBytes, _ := json.Marshal(portingRequestContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		reqStatus,
		string(portingRequestContentBytes),
	}
}

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildReqPTokensInst(
	uniquePortingID string,
	tokenID string,
	incogAddressStr string,
	portingAmount uint64,
	portingProof string,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	reqPTokenContent := metadata.PortalRequestPTokensContent{
		UniquePortingID: uniquePortingID,
		TokenID:         tokenID,
		IncogAddressStr: incogAddressStr,
		PortingAmount:   portingAmount,
		PortingProof:    portingProof,
		TxReqID:         txReqID,
		ShardID:         shardID,
	}
	reqPTokenContentBytes, _ := json.Marshal(reqPTokenContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(reqPTokenContentBytes),
	}
}


func buildCustodianWithdrawInst(
	metaType int,
	shardID byte,
	reqStatus string,
	paymentAddress string,
	amount uint64,
	remainFreeCollateral uint64,
	txReqID common.Hash,
) []string {
	content := metadata.PortalCustodianWithdrawRequestContent{
		PaymentAddress: paymentAddress,
		Amount: amount,
		RemainFreeCollateral: remainFreeCollateral,
		TxReqID:          txReqID,
		ShardID: shardID,
	}

	contentBytes, _ := json.Marshal(content)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		reqStatus,
		string(contentBytes),
	}
}

// buildInstructionsForCustodianDeposit builds instruction for custodian deposit action
func (blockchain *BlockChain) buildInstructionsForCustodianDeposit(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalCustodianDepositAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}

	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForCustodianDeposit]: Current Portal state is null.")
		// need to refund collateral to custodian
		inst := buildCustodianDepositInst(
			actionData.Meta.IncogAddressStr,
			actionData.Meta.DepositedAmount,
			actionData.Meta.RemoteAddresses,
			actionData.Meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalCustodianDepositRefundChainStatus,
		)
		return [][]string{inst}, nil
	}
	meta := actionData.Meta

	keyCustodianState := lvdb.NewCustodianStateKey(beaconHeight, meta.IncogAddressStr)

	if currentPortalState.CustodianPoolState[keyCustodianState] == nil {
		// new custodian
		newCustodian, _ := NewCustodianState(
			meta.IncogAddressStr, meta.DepositedAmount, meta.DepositedAmount,
			nil, nil,
			meta.RemoteAddresses, 0)
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	} else {
		// custodian deposited before
		// update state of the custodian
		custodian := currentPortalState.CustodianPoolState[keyCustodianState]
		totalCollateral := custodian.TotalCollateral + meta.DepositedAmount
		freeCollateral := custodian.FreeCollateral + meta.DepositedAmount
		holdingPubTokens := custodian.HoldingPubTokens
		lockedAmountCollateral := custodian.LockedAmountCollateral
		rewardAmount := custodian.RewardAmount
		remoteAddresses := custodian.RemoteAddresses
		for _, address := range meta.RemoteAddresses {
			if existedAddr, _ := lvdb.GetRemoteAddressByTokenID(remoteAddresses, address.PTokenID); existedAddr == "" {
				remoteAddresses = append(remoteAddresses, address)
			}
		}

		newCustodian, _ := NewCustodianState(meta.IncogAddressStr, totalCollateral, freeCollateral,
			holdingPubTokens, lockedAmountCollateral, remoteAddresses, rewardAmount)
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	}

	inst := buildCustodianDepositInst(
		actionData.Meta.IncogAddressStr,
		actionData.Meta.DepositedAmount,
		actionData.Meta.RemoteAddresses,
		actionData.Meta.Type,
		shardID,
		actionData.TxReqID,
		common.PortalCustodianDepositAcceptedChainStatus,
	)
	return [][]string{inst}, nil
}

func (blockchain *BlockChain) buildInstructionsForPortingRequest(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("Porting request: an error occurred while decoding content string of portal porting request action: %+v", err)
		return [][]string{}, nil
	}

	var actionData metadata.PortalUserRegisterAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("Porting request: an error occurred while unmarshal portal porting request action: %+v", err)
		return [][]string{}, nil
	}

	if currentPortalState == nil {
		Logger.log.Warn("Porting request: Current Portal state is null")
		return [][]string{}, nil
	}

	db := blockchain.GetDatabase()


	keyPortingRequest := lvdb.NewPortingRequestKey(actionData.Meta.UniqueRegisterId)
	//check unique id from record from db
	portingRequestKeyExist, err := db.GetItemPortalByKey([]byte(keyPortingRequest))

	if err != nil {
		Logger.log.Errorf("Porting request: Get item portal by prefix error: %+v", err)

		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	if portingRequestKeyExist != nil {
		Logger.log.Errorf("Porting request: Porting request exist, key %v", keyPortingRequest)
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	waitingPortingRequestKey := lvdb.NewWaitingPortingReqKey(beaconHeight, actionData.Meta.UniqueRegisterId)
	if _, ok := currentPortalState.WaitingPortingRequests[waitingPortingRequestKey]; ok {
		Logger.log.Errorf("Porting request: Waiting porting request exist, key %v", waitingPortingRequestKey)
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//get exchange rates
	exchangeRatesKey := lvdb.NewFinalExchangeRatesKey(beaconHeight)
	exchangeRatesState, ok := currentPortalState.FinalExchangeRates[exchangeRatesKey]
	if !ok {
		Logger.log.Errorf("Porting request, exchange rates not found")
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	if len(currentPortalState.CustodianPoolState) <= 0 {
		Logger.log.Errorf("Porting request: Custodian not found")
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	var sortCustodianStateByFreeCollateral []CustodianStateSlice
	_ = sortCustodianByAmountAscent(actionData.Meta, currentPortalState.CustodianPoolState, &sortCustodianStateByFreeCollateral)

	if len(sortCustodianStateByFreeCollateral) <= 0 {
		Logger.log.Errorf("Porting request, custodian not found")

		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//validation porting fees
	exchangePortingFees, err := CalMinPortingFee(actionData.Meta.RegisterAmount, actionData.Meta.PTokenId, exchangeRatesState)
	if err != nil {
		Logger.log.Errorf("Calculate Porting fee is error %v", err)
		return [][]string{}, nil
	}

	Logger.log.Infof("Porting request, porting fees need %v", exchangePortingFees)

	if actionData.Meta.PortingFee < exchangePortingFees {
		Logger.log.Errorf("Porting request, Porting fees is wrong")

		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//pick one
	pickCustodianResult, _ := pickSingleCustodian(actionData.Meta, exchangeRatesState, sortCustodianStateByFreeCollateral, currentPortalState)

	Logger.log.Infof("Porting request, pick single custodian result %v", len(pickCustodianResult))
	//pick multiple
	if len(pickCustodianResult) == 0 {
		pickCustodianResult, _ = pickMultipleCustodian(actionData.Meta, exchangeRatesState, sortCustodianStateByFreeCollateral, currentPortalState)
		Logger.log.Infof("Porting request, pick multiple custodian result %v", len(pickCustodianResult))
	}
	//end
	if len(pickCustodianResult) == 0 {
		Logger.log.Errorf("Porting request, custodian not found")
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			pickCustodianResult,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	//verify total amount
	var totalPToken uint64 = 0
	for _, eachCustodian := range pickCustodianResult {
		totalPToken = totalPToken + eachCustodian.Amount
	}

	if totalPToken != actionData.Meta.RegisterAmount {
		Logger.log.Errorf("Porting request, total custodian picked difference with total input PToken %v != %v", actionData.Meta.RegisterAmount, totalPToken)

		Logger.log.Errorf("Porting request, custodian not found")
		inst := buildRequestPortingInst(
			actionData.Meta.Type,
			shardID,
			common.PortalPortingRequestRejectedChainStatus,
			actionData.Meta.UniqueRegisterId,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PTokenId,
			actionData.Meta.RegisterAmount,
			actionData.Meta.PortingFee,
			nil,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}


	inst := buildRequestPortingInst(
		actionData.Meta.Type,
		shardID,
		common.PortalPortingRequestAcceptedChainStatus,
		actionData.Meta.UniqueRegisterId,
		actionData.Meta.IncogAddressStr,
		actionData.Meta.PTokenId,
		actionData.Meta.RegisterAmount,
		actionData.Meta.PortingFee,
		pickCustodianResult,
		actionData.TxReqID,
	) //return  metadata.PortalPortingRequestContent at instruct[3]

	newPortingRequestStateWaiting, err := NewPortingRequestState(
		actionData.Meta.UniqueRegisterId,
		actionData.TxReqID,
		actionData.Meta.PTokenId,
		actionData.Meta.IncogAddressStr,
		actionData.Meta.RegisterAmount,
		pickCustodianResult,
		actionData.Meta.PortingFee,
		common.PortalPortingReqWaitingStatus,
		beaconHeight+1,
	)

	keyWaitingPortingRequest := lvdb.NewWaitingPortingReqKey(beaconHeight, actionData.Meta.UniqueRegisterId)
	currentPortalState.WaitingPortingRequests[keyWaitingPortingRequest] = newPortingRequestStateWaiting

	return [][]string{inst}, nil
}

// buildInstructionsForCustodianDeposit builds instruction for custodian deposit action
func (blockchain *BlockChain) buildInstructionsForReqPTokens(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {

	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalRequestPTokensAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal custodian deposit action: %+v", err)
		return [][]string{}, nil
	}
	meta := actionData.Meta

	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForCustodianDeposit]: Current Portal state is null.")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check meta.UniquePortingID is in waiting PortingRequests list in portal state or not
	portingID := meta.UniquePortingID
	keyWaitingPortingRequest := lvdb.NewWaitingPortingReqKey(beaconHeight, portingID)
	waitingPortingRequest := currentPortalState.WaitingPortingRequests[keyWaitingPortingRequest]
	if waitingPortingRequest == nil {
		Logger.log.Errorf("PortingID is not existed in waiting porting requests list")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	db := blockchain.GetDatabase()

	// check porting request status of portingID from db
	portingReqStatus, err := db.GetPortingRequestStatusByPortingID(meta.UniquePortingID)
	if err != nil {
		Logger.log.Errorf("Can not get porting req status for portingID %v, %v\n", meta.UniquePortingID, err)
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if portingReqStatus != common.PortalPortingReqWaitingStatus {
		Logger.log.Errorf("PortingID status invalid")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check tokenID
	if meta.TokenID != waitingPortingRequest.TokenID {
		Logger.log.Errorf("TokenID is not correct in portingID req")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check porting amount
	if meta.PortingAmount != waitingPortingRequest.Amount {
		Logger.log.Errorf("PortingAmount is not correct in portingID req")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if meta.TokenID == common.PortalBTCIDStr {
		//todo:
	} else if meta.TokenID == common.PortalBNBIDStr {
		// parse PortingProof in meta
		txProofBNB, err := bnb.ParseBNBProofFromB64EncodeStr(meta.PortingProof)
		if err != nil {
			Logger.log.Errorf("PortingProof is invalid %v\n", err)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		isValid, err := txProofBNB.Verify(db)
		if !isValid || err != nil {
			Logger.log.Errorf("Verify txProofBNB failed %v", err)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// parse Tx from Data in txProofBNB
		txBNB, err := bnb.ParseTxFromData(txProofBNB.Proof.Data)
		if err != nil {
			Logger.log.Errorf("Data in PortingProof is invalid %v", err)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check memo attach portingID req:
		memo := txBNB.Memo
		memoBytes, err2 := base64.StdEncoding.DecodeString(memo)
		if err2 != nil {
			Logger.log.Errorf("Can not decode memo in tx bnb proof", err2)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		var portingMemo PortingMemoBNB
		err2 = json.Unmarshal(memoBytes, &portingMemo)
		if err2 != nil {
			Logger.log.Errorf("Can not unmarshal memo in tx bnb proof", err2)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		if portingMemo.PortingID != meta.UniquePortingID {
			Logger.log.Errorf("PortingId in memoTx is not matched with portingID in metadata", err2)
			inst := buildReqPTokensInst(
				meta.UniquePortingID,
				meta.TokenID,
				meta.IncogAddressStr,
				meta.PortingAmount,
				meta.PortingProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqPTokensRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check whether amount transfer in txBNB is equal porting amount or not
		// check receiver and amount in tx
		// get list matching custodians in waitingPortingRequest
		custodians := waitingPortingRequest.Custodians
		outputs := txBNB.Msgs[0].(msg.SendMsg).Outputs
		for _, cusDetail := range custodians {
			remoteAddressNeedToBeTransfer := cusDetail.RemoteAddress
			amountNeedToBeTransfer := cusDetail.Amount
			amountNeedToBeTransferInBNB := convertIncPBNBAmountToExternalBNBAmount(int64(amountNeedToBeTransfer))

			isChecked := false
			for _, out := range outputs {
				addr, _ := bnb.GetAccAddressString(&out.Address, blockchain.config.ChainParams.BNBRelayingHeaderChainID)
				if addr != remoteAddressNeedToBeTransfer {
					Logger.log.Errorf("[portal] remoteAddressNeedToBeTransfer: %v - addr: %v\n", remoteAddressNeedToBeTransfer, addr)
					continue
				}

				// calculate amount that was transferred to custodian's remote address
				amountTransfer := int64(0)
				for _, coin := range out.Coins {
					if coin.Denom == bnb.DenomBNB {
						amountTransfer += coin.Amount
						// note: log error for debug
						Logger.log.Errorf("TxProof-BNB coin.Amount %d",
							coin.Amount)
					}
				}
				if amountTransfer < amountNeedToBeTransferInBNB {
					Logger.log.Errorf("TxProof-BNB is invalid - Amount transfer to %s must be equal to or greater than %d, but got %d",
						addr, amountNeedToBeTransferInBNB, amountTransfer)
					inst := buildReqPTokensInst(
						meta.UniquePortingID,
						meta.TokenID,
						meta.IncogAddressStr,
						meta.PortingAmount,
						meta.PortingProof,
						meta.Type,
						shardID,
						actionData.TxReqID,
						common.PortalReqPTokensRejectedChainStatus,
					)
					return [][]string{inst}, nil
				} else {
					isChecked = true
					break
				}
			}
			if !isChecked {
				Logger.log.Errorf("TxProof-BNB is invalid - Receiver address is invalid, expected %v",
					remoteAddressNeedToBeTransfer)
				inst := buildReqPTokensInst(
					meta.UniquePortingID,
					meta.TokenID,
					meta.IncogAddressStr,
					meta.PortingAmount,
					meta.PortingProof,
					meta.Type,
					shardID,
					actionData.TxReqID,
					common.PortalReqPTokensRejectedChainStatus,
				)
				return [][]string{inst}, nil
			}
		}

		inst := buildReqPTokensInst(
			actionData.Meta.UniquePortingID,
			actionData.Meta.TokenID,
			actionData.Meta.IncogAddressStr,
			actionData.Meta.PortingAmount,
			actionData.Meta.PortingProof,
			actionData.Meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensAcceptedChainStatus,
		)

		// remove waiting porting request from currentPortalState
		removeWaitingPortingReqByKey(keyWaitingPortingRequest, currentPortalState)
		return [][]string{inst}, nil
	} else {
		Logger.log.Errorf("TokenID is not supported currently on Portal")
		inst := buildReqPTokensInst(
			meta.UniquePortingID,
			meta.TokenID,
			meta.IncogAddressStr,
			meta.PortingAmount,
			meta.PortingProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqPTokensRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	return [][]string{}, nil
}

func (blockchain *BlockChain) buildInstructionsForExchangeRates(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while decoding content string of portal exchange rates action: %+v", err)
		return [][]string{}, nil
	}

	var actionData metadata.PortalExchangeRatesAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshal portal exchange rates action: %+v", err)
		return [][]string{}, nil
	}

	exchangeRatesKey := lvdb.NewExchangeRatesRequestKey(
		beaconHeight+1,
		actionData.TxReqID.String(),
	)

	db := blockchain.GetDatabase()
	//check key from db
	exchangeRatesKeyExist, err := db.GetItemPortalByKey([]byte(exchangeRatesKey))

	if err != nil {
		Logger.log.Errorf("ERROR: Get exchange rates error: %+v", err)

		portalExchangeRatesContent := metadata.PortalExchangeRatesContent{
			SenderAddress:   actionData.Meta.SenderAddress,
			Rates:           actionData.Meta.Rates,
			TxReqID:         actionData.TxReqID,
			LockTime:        actionData.LockTime,
			UniqueRequestId: exchangeRatesKey,
		}

		portalExchangeRatesContentBytes, _ := json.Marshal(portalExchangeRatesContent)

		inst := []string{
			strconv.Itoa(metaType),
			strconv.Itoa(int(shardID)),
			common.PortalExchangeRatesRejectedStatus,
			string(portalExchangeRatesContentBytes),
		}

		return [][]string{inst}, nil
	}

	if exchangeRatesKeyExist != nil {
		Logger.log.Errorf("ERROR: exchange rates key is duplicated")

		portalExchangeRatesContent := metadata.PortalExchangeRatesContent{
			SenderAddress:   actionData.Meta.SenderAddress,
			Rates:           actionData.Meta.Rates,
			TxReqID:         actionData.TxReqID,
			LockTime:        actionData.LockTime,
			UniqueRequestId: exchangeRatesKey,
		}

		portalExchangeRatesContentBytes, _ := json.Marshal(portalExchangeRatesContent)

		inst := []string{
			strconv.Itoa(metaType),
			strconv.Itoa(int(shardID)),
			common.PortalExchangeRatesRejectedStatus,
			string(portalExchangeRatesContentBytes),
		}

		return [][]string{inst}, nil
	}

	//success
	portalExchangeRatesContent := metadata.PortalExchangeRatesContent{
		SenderAddress:   actionData.Meta.SenderAddress,
		Rates:           actionData.Meta.Rates,
		TxReqID:         actionData.TxReqID,
		LockTime:        actionData.LockTime,
		UniqueRequestId: exchangeRatesKey,
	}

	portalExchangeRatesContentBytes, _ := json.Marshal(portalExchangeRatesContent)

	inst := []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		common.PortalExchangeRatesSuccessStatus,
		string(portalExchangeRatesContentBytes),
	}

	return [][]string{inst}, nil
}

// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildRedeemRequestInst(
	uniqueRedeemID string,
	tokenID string,
	redeemAmount uint64,
	incAddressStr string,
	remoteAddress string,
	redeemFee uint64,
	matchingCustodianDetail []*lvdb.MatchingRedeemCustodianDetail,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	redeemRequestContent := metadata.PortalRedeemRequestContent{
		UniqueRedeemID:          uniqueRedeemID,
		TokenID:                 tokenID,
		RedeemAmount:            redeemAmount,
		RedeemerIncAddressStr:   incAddressStr,
		RemoteAddress:           remoteAddress,
		MatchingCustodianDetail: matchingCustodianDetail,
		RedeemFee:               redeemFee,
		TxReqID:                 txReqID,
		ShardID:                 shardID,
	}
	redeemRequestContentBytes, _ := json.Marshal(redeemRequestContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(redeemRequestContentBytes),
	}
}

// buildInstructionsForRedeemRequest builds instruction for redeem request action
func (blockchain *BlockChain) buildInstructionsForRedeemRequest(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal redeem request action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalRedeemRequestAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal redeem request action: %+v", err)
		return [][]string{}, nil
	}

	meta := actionData.Meta
	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForRedeemRequest]: Current Portal state is null.")
		// need to mint ptoken to user
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	redeemID := meta.UniqueRedeemID

	// check uniqueRedeemID is existed waitingRedeem list or not
	keyWaitingRedeemRequest := lvdb.NewWaitingRedeemReqKey(beaconHeight, redeemID)
	waitingRedeemRequest := currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest]
	if waitingRedeemRequest != nil {
		Logger.log.Errorf("RedeemID is existed in waiting redeem requests list %v\n", redeemID)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	db := blockchain.GetDatabase()

	// check uniqueRedeemID is existed in db or not
	redeemRequestBytes, err := db.GetRedeemRequestByRedeemID(meta.UniqueRedeemID)
	if err != nil {
		Logger.log.Errorf("Can not get redeem req status for redeemID %v, %v\n", meta.UniqueRedeemID, err)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	} else if len(redeemRequestBytes) > 0 {
		Logger.log.Errorf("RedeemID is existed in redeem requests list in db %v\n", redeemID)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// get tokenID from redeemTokenID
	tokenID := meta.TokenID

	// check redeem fee
	exchangeRateKey := lvdb.NewFinalExchangeRatesKey(beaconHeight)
	if currentPortalState.FinalExchangeRates[exchangeRateKey] == nil {
		Logger.log.Errorf("Can not get exchange rate at beaconHeight %v\n", beaconHeight)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	minRedeemFee, err := calMinRedeemFee(meta.RedeemAmount, tokenID, currentPortalState.FinalExchangeRates[exchangeRateKey])
	if err != nil {
		Logger.log.Errorf("Error when calculating minimum redeem fee %v\n", err)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if meta.RedeemFee < minRedeemFee {
		Logger.log.Errorf("Redeem fee is invalid, minRedeemFee %v, but get %v\n", minRedeemFee, meta.RedeemFee)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// pick custodian(s) who holding public token to return user
	matchingCustodiansDetail, err := pickupCustodianForRedeem(meta.RedeemAmount, tokenID, currentPortalState)
	if err != nil {
		Logger.log.Errorf("Error when pick up custodian for redeem %v\n", err)
		inst := buildRedeemRequestInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.RedeemAmount,
			meta.RedeemerIncAddressStr,
			meta.RemoteAddress,
			meta.RedeemFee,
			nil,
			meta.Type,
			actionData.ShardID,
			actionData.TxReqID,
			common.PortalRedeemRequestRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// update custodian state (holding public tokens)
	for _, cus := range matchingCustodiansDetail {
		custodianStateKey := lvdb.NewCustodianStateKey(beaconHeight, cus.IncAddress)
		if currentPortalState.CustodianPoolState[custodianStateKey].HoldingPubTokens[tokenID] < cus.Amount {
			Logger.log.Errorf("Amount holding public tokens is less than matching redeem amount")
			inst := buildRedeemRequestInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.RedeemAmount,
				meta.RedeemerIncAddressStr,
				meta.RemoteAddress,
				meta.RedeemFee,
				nil,
				meta.Type,
				actionData.ShardID,
				actionData.TxReqID,
				common.PortalRedeemRequestRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}
		currentPortalState.CustodianPoolState[custodianStateKey].HoldingPubTokens[tokenID] -= cus.Amount
	}

	// add to waiting Redeem list
	redeemRequest, _ := NewRedeemRequestState(
		meta.UniqueRedeemID,
		actionData.TxReqID,
		meta.TokenID,
		meta.RedeemerIncAddressStr,
		meta.RemoteAddress,
		meta.RedeemAmount,
		matchingCustodiansDetail,
		meta.RedeemFee,
		beaconHeight + 1,
	)
	currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest] = redeemRequest

	Logger.log.Infof("[Portal] Build accepted instruction for redeem request")
	inst := buildRedeemRequestInst(
		meta.UniqueRedeemID,
		meta.TokenID,
		meta.RedeemAmount,
		meta.RedeemerIncAddressStr,
		meta.RemoteAddress,
		meta.RedeemFee,
		matchingCustodiansDetail,
		meta.Type,
		actionData.ShardID,
		actionData.TxReqID,
		common.PortalRedeemRequestAcceptedChainStatus,
	)
	return [][]string{inst}, nil
}


/**
	Validation:
		- verify each instruct belong shard
		- check amount < fee collateral
		- build PortalCustodianWithdrawRequestContent to send beacon
 */
func (blockchain *BlockChain) buildInstructionsForCustodianWithdraw(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("Have an error occurred while decoding content string of custodian withdraw request action: %+v", err)
		return [][]string{}, nil
	}

	var actionData metadata.PortalCustodianWithdrawRequestAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("Have an error occurred while unmarshal  custodian withdraw request action: %+v", err)
		return [][]string{}, nil
	}

	if currentPortalState == nil {
		Logger.log.Warn("Current Portal state is null")
		return [][]string{}, nil
	}

	db := blockchain.GetDatabase()

	//check custodian withdraw request
	custodianWithdrawRequestKey := lvdb.NewCustodianWithdrawRequest(actionData.TxReqID.String())
	custodianWithdrawRequestKeyExist, err := db.GetItemPortalByKey([]byte(custodianWithdrawRequestKey))

	if err != nil {
		Logger.log.Errorf("Custodian withdraw is exist %+v", err)

		inst := buildCustodianWithdrawInst(
			actionData.Meta.Type,
			shardID,
			common.PortalCustodianWithdrawRequestRejectedStatus,
			actionData.Meta.PaymentAddress,
			actionData.Meta.Amount,
			0,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	if custodianWithdrawRequestKeyExist != nil {
		Logger.log.Errorf("Custodian withdraw key is duplicated")

		inst := buildCustodianWithdrawInst(
			actionData.Meta.Type,
			shardID,
			common.PortalCustodianWithdrawRequestRejectedStatus,
			actionData.Meta.PaymentAddress,
			actionData.Meta.Amount,
			0,
			actionData.TxReqID,
		)

		return [][]string{inst}, nil
	}

	if len(currentPortalState.CustodianPoolState) <= 0 {
		Logger.log.Errorf("Custodian state is empty")

		inst := buildCustodianWithdrawInst(
			actionData.Meta.Type,
			shardID,
			common.PortalCustodianWithdrawRequestRejectedStatus,
			actionData.Meta.PaymentAddress,
			actionData.Meta.Amount,
			0,
			actionData.TxReqID,
		)
		return [][]string{inst}, nil
	}

	custodianKey := lvdb.NewCustodianStateKey(beaconHeight, actionData.Meta.PaymentAddress)
	custodian, ok := currentPortalState.CustodianPoolState[custodianKey]

	if !ok {
		Logger.log.Errorf("Custodian not found")

		inst := buildCustodianWithdrawInst(
			actionData.Meta.Type,
			shardID,
			common.PortalCustodianWithdrawRequestRejectedStatus,
			actionData.Meta.PaymentAddress,
			actionData.Meta.Amount,
			0,
			actionData.TxReqID,
		)
		return [][]string{inst}, nil
	}

	if actionData.Meta.Amount > custodian.FreeCollateral {
		Logger.log.Errorf("Free Collateral is not enough PRV")

		inst := buildCustodianWithdrawInst(
			actionData.Meta.Type,
			shardID,
			common.PortalCustodianWithdrawRequestRejectedStatus,
			actionData.Meta.PaymentAddress,
			actionData.Meta.Amount,
			0,
			actionData.TxReqID,
		)
		return [][]string{inst}, nil
	}
	//withdraw
	remainFreeCollateral := custodian.FreeCollateral - actionData.Meta.Amount
	totalFreeCollateral := custodian.TotalCollateral - actionData.Meta.Amount

	inst := buildCustodianWithdrawInst(
		actionData.Meta.Type,
		shardID,
		common.PortalCustodianWithdrawRequestAcceptedStatus,
		actionData.Meta.PaymentAddress,
		actionData.Meta.Amount,
		remainFreeCollateral,
		actionData.TxReqID,
	)

	//update free collateral custodian
	custodian.FreeCollateral = remainFreeCollateral
	custodian.TotalCollateral = totalFreeCollateral
	currentPortalState.CustodianPoolState[custodianKey] = custodian
	return [][]string{inst}, nil
}
// beacon build new instruction from instruction received from ShardToBeaconBlock
func buildReqUnlockCollateralInst(
	uniqueRedeemID string,
	tokenID string,
	custodianAddressStr string,
	redeemAmount uint64,
	unlockAmount uint64,
	redeemProof string,
	metaType int,
	shardID byte,
	txReqID common.Hash,
	status string,
) []string {
	reqUnlockCollateralContent := metadata.PortalRequestUnlockCollateralContent{
		UniqueRedeemID:      uniqueRedeemID,
		TokenID:             tokenID,
		CustodianAddressStr: custodianAddressStr,
		RedeemAmount:        redeemAmount,
		UnlockAmount: unlockAmount,
		RedeemProof:         redeemProof,
		TxReqID:             txReqID,
		ShardID:             shardID,
	}
	reqUnlockCollateralContentBytes, _ := json.Marshal(reqUnlockCollateralContent)
	return []string{
		strconv.Itoa(metaType),
		strconv.Itoa(int(shardID)),
		status,
		string(reqUnlockCollateralContentBytes),
	}
}

// buildInstructionsForReqUnlockCollateral builds instruction for custodian deposit action
func (blockchain *BlockChain) buildInstructionsForReqUnlockCollateral(
	contentStr string,
	shardID byte,
	metaType int,
	currentPortalState *CurrentPortalState,
	beaconHeight uint64,
) ([][]string, error) {

	// parse instruction
	actionContentBytes, err := base64.StdEncoding.DecodeString(contentStr)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while decoding content string of portal request unlock collateral action: %+v", err)
		return [][]string{}, nil
	}
	var actionData metadata.PortalRequestUnlockCollateralAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occured while unmarshal portal request unlock collateral action: %+v", err)
		return [][]string{}, nil
	}
	meta := actionData.Meta

	if currentPortalState == nil {
		Logger.log.Warn("WARN - [buildInstructionsForReqUnlockCollateral]: Current Portal state is null.")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check meta.UniqueRedeemID is in waiting RedeemRequests list in portal state or not
	redeemID := meta.UniqueRedeemID
	keyWaitingRedeemRequest := lvdb.NewWaitingRedeemReqKey(beaconHeight, redeemID)
	waitingRedeemRequest := currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest]
	if waitingRedeemRequest == nil {
		Logger.log.Errorf("redeemID is not existed in waiting redeem requests list")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	db := blockchain.GetDatabase()

	// check status of request unlock collateral by redeemID
	redeemReqStatusBytes, err := db.GetRedeemRequestByRedeemID(redeemID)
	if err != nil {
		Logger.log.Errorf("Can not get redeem request by redeemID from db %v\n", err)
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}
	var redeemRequest metadata.PortalRedeemRequestStatus
	err = json.Unmarshal(redeemReqStatusBytes, &redeemRequest)
	if err != nil {
		Logger.log.Errorf("Can not unmarshal redeem request %v\n", err)
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	if redeemRequest.Status != common.PortalRedeemReqWaitingStatus {
		Logger.log.Errorf("Redeem request %v has invalid status %v\n", redeemID, redeemRequest.Status)
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}

	// check tokenID
	if meta.TokenID != waitingRedeemRequest.TokenID {
		Logger.log.Errorf("TokenID is not correct in redeemID req")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,
		)
		return [][]string{inst}, nil
	}


	// check redeem amount of matching custodian
	amountMatchingCustodian := uint64(0)
	for _, cus := range waitingRedeemRequest.Custodians {
		if cus.IncAddress == meta.CustodianAddressStr {
			amountMatchingCustodian = cus.Amount
			break
		}
	}

	if meta.RedeemAmount != amountMatchingCustodian {
		Logger.log.Errorf("RedeemAmount is not correct in redeemID req")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,

		)
		return [][]string{inst}, nil
	}


	// validate proof and memo in tx
	if meta.TokenID == common.PortalBTCIDStr {
		//todo:
	} else if meta.TokenID == common.PortalBNBIDStr {
		// parse PortingProof in meta
		txProofBNB, err := bnb.ParseBNBProofFromB64EncodeStr(meta.RedeemProof)
		if err != nil {
			Logger.log.Errorf("RedeemProof is invalid %v\n", err)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		isValid, err := txProofBNB.Verify(db)
		if !isValid || err != nil {
			Logger.log.Errorf("Verify txProofBNB failed %v", err)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// parse Tx from Data in txProofBNB
		txBNB, err := bnb.ParseTxFromData(txProofBNB.Proof.Data)
		if err != nil {
			Logger.log.Errorf("Data in RedeemProof is invalid %v", err)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check memo attach redeemID req:
		memo := txBNB.Memo
		Logger.log.Infof("[buildInstructionsForReqUnlockCollateral] memo: %v\n", memo)
		memoBytes, err2 := base64.StdEncoding.DecodeString(memo)
		if err2 != nil {
			Logger.log.Errorf("Can not decode memo in tx bnb proof", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}
		Logger.log.Infof("[buildInstructionsForReqUnlockCollateral] memoBytes: %v\n", memoBytes)

		var redeemMemo RedeemMemoBNB
		err2 = json.Unmarshal(memoBytes, &redeemMemo)
		if err2 != nil {
			Logger.log.Errorf("Can not unmarshal memo in tx bnb proof", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		if redeemMemo.RedeemID != meta.UniqueRedeemID {
			Logger.log.Errorf("PortingId in memoTx is not matched with redeemID in metadata", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}
		if redeemMemo.CustodianIncognitoAddress != meta.CustodianAddressStr {
			Logger.log.Errorf("CustodianIncognitoAddress in memoTx is not matched with CustodianIncognitoAddress in metadata", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// check whether amount transfer in txBNB is equal redeem amount or not
		// check receiver and amount in tx
		// get list matching custodians in waitingRedeemRequest

		outputs := txBNB.Msgs[0].(msg.SendMsg).Outputs

		remoteAddressNeedToBeTransfer := waitingRedeemRequest.RedeemerRemoteAddress
		amountNeedToBeTransfer := meta.RedeemAmount
		amountNeedToBeTransferInBNB := convertIncPBNBAmountToExternalBNBAmount(int64(amountNeedToBeTransfer))

		isChecked := false
		for _, out := range outputs {
			addr, _ := bnb.GetAccAddressString(&out.Address, blockchain.config.ChainParams.BNBRelayingHeaderChainID)
			if addr != remoteAddressNeedToBeTransfer {
				continue
			}

			// calculate amount that was transferred to custodian's remote address
			amountTransfer := int64(0)
			for _, coin := range out.Coins {
				if coin.Denom == bnb.DenomBNB {
					amountTransfer += coin.Amount
					// note: log error for debug
					Logger.log.Errorf("TxProof-BNB coin.Amount %d",
						coin.Amount)
				}
			}
			if amountTransfer < amountNeedToBeTransferInBNB {
				Logger.log.Errorf("TxProof-BNB is invalid - Amount transfer to %s must be equal to or greater than %d, but got %d",
					addr, amountNeedToBeTransferInBNB, amountTransfer)
				inst := buildReqUnlockCollateralInst(
					meta.UniqueRedeemID,
					meta.TokenID,
					meta.CustodianAddressStr,
					meta.RedeemAmount,
					0,
					meta.RedeemProof,
					meta.Type,
					shardID,
					actionData.TxReqID,
					common.PortalReqUnlockCollateralRejectedChainStatus,
				)
				return [][]string{inst}, nil
			} else {
				isChecked = true
				break
			}
		}

		if !isChecked{
			Logger.log.Errorf("TxProof-BNB is invalid - Receiver address is invalid, expected %v",
				remoteAddressNeedToBeTransfer)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// get tokenID from redeemTokenID
		tokenID := meta.TokenID

		// update custodian state (FreeCollateral, LockedAmountCollateral)
		custodianStateKey := lvdb.NewCustodianStateKey(beaconHeight, meta.CustodianAddressStr)
		finalExchangeRateKey := lvdb.NewFinalExchangeRatesKey(beaconHeight)
		unlockAmount, err2 := updateFreeCollateralCustodian(
			currentPortalState.CustodianPoolState[custodianStateKey],
			meta.RedeemAmount, tokenID,
			currentPortalState.FinalExchangeRates[finalExchangeRateKey])
		if err2 != nil {
			Logger.log.Errorf("Error when update free collateral amount for custodian", err2)
			inst := buildReqUnlockCollateralInst(
				meta.UniqueRedeemID,
				meta.TokenID,
				meta.CustodianAddressStr,
				meta.RedeemAmount,
				0,
				meta.RedeemProof,
				meta.Type,
				shardID,
				actionData.TxReqID,
				common.PortalReqUnlockCollateralRejectedChainStatus,
			)
			return [][]string{inst}, nil
		}

		// update redeem request state in WaitingRedeemRequest (remove custodian from matchingCustodianDetail)
		currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest].Custodians, _ = removeCustodianFromMatchingRedeemCustodians(
			currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest].Custodians, meta.CustodianAddressStr)

		// remove redeem request from WaitingRedeemRequest list when all matching custodians return public token to user
		// when list matchingCustodianDetail is empty
		if len(currentPortalState.WaitingRedeemRequests[keyWaitingRedeemRequest].Custodians) == 0 {
			delete(currentPortalState.WaitingRedeemRequests, keyWaitingRedeemRequest)
		}

		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			unlockAmount,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralAcceptedChainStatus,
		)

		return [][]string{inst}, nil
	} else {
		Logger.log.Errorf("TokenID is not supported currently on Portal")
		inst := buildReqUnlockCollateralInst(
			meta.UniqueRedeemID,
			meta.TokenID,
			meta.CustodianAddressStr,
			meta.RedeemAmount,
			0,
			meta.RedeemProof,
			meta.Type,
			shardID,
			actionData.TxReqID,
			common.PortalReqUnlockCollateralRejectedChainStatus,

		)
		return [][]string{inst}, nil
	}

	return [][]string{}, nil
}
