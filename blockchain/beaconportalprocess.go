package blockchain

import (
	"encoding/json"
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/database"
	"github.com/incognitochain/incognito-chain/database/lvdb"
	"github.com/incognitochain/incognito-chain/metadata"
	"sort"
	"strconv"
)

func (blockchain *BlockChain) processPortalInstructions(block *BeaconBlock, bd *[]database.BatchData) error {
	beaconHeight := block.Header.Height - 1
	db := blockchain.GetDatabase()

	currentPortalState, err := InitCurrentPortalStateFromDB(db, beaconHeight)
	if err != nil {
		Logger.log.Error(err)
		return nil
	}

	// re-use update info of bridge
	updatingInfoByTokenID := map[common.Hash]UpdatingInfo{}

	for _, inst := range block.Body.Instructions {
		if len(inst) < 4 {
			continue // Not error, just not Portal instruction
		}

		var err error

		switch inst[0] {
		case strconv.Itoa(metadata.PortalCustodianDepositMeta):
			err = blockchain.processPortalCustodianDeposit(beaconHeight, inst, currentPortalState)
		case strconv.Itoa(metadata.PortalUserRegisterMeta):
			err = blockchain.processPortalUserRegister(beaconHeight, inst, currentPortalState)
		case strconv.Itoa(metadata.PortalUserRequestPTokenMeta):
			err = blockchain.processPortalUserReqPToken(beaconHeight, inst, currentPortalState, updatingInfoByTokenID)
		case strconv.Itoa(metadata.PortalExchangeRatesMeta):
			err = blockchain.processPortalExchangeRates(beaconHeight, inst, currentPortalState)
		}

		if err != nil {
			Logger.log.Error(err)
			return nil
		}
	}

	//todo: check timeout register porting via beacon height
	// all request timeout ? unhold

	//save final exchangeRates
	err = blockchain.pickExchangesRatesFinal(beaconHeight, currentPortalState)
	if err != nil {
		Logger.log.Error(err)
		return nil
	}

	// update info of bridge portal token
	for _, updatingInfo := range updatingInfoByTokenID {
		var updatingAmt uint64
		var updatingType string
		if updatingInfo.countUpAmt > updatingInfo.deductAmt {
			updatingAmt = updatingInfo.countUpAmt - updatingInfo.deductAmt
			updatingType = "+"
		}
		if updatingInfo.countUpAmt < updatingInfo.deductAmt {
			updatingAmt = updatingInfo.deductAmt - updatingInfo.countUpAmt
			updatingType = "-"
		}
		err := db.UpdateBridgeTokenInfo(
			updatingInfo.tokenID,
			updatingInfo.externalTokenID,
			updatingInfo.isCentralized,
			updatingAmt,
			updatingType,
			bd,
		)
		if err != nil {
			return err
		}
	}

	// store updated currentPortalState to leveldb with new beacon height
	err = storePortalStateToDB(db, beaconHeight+1, currentPortalState)
	if err != nil {
		Logger.log.Error(err)
	}

	return nil
}

func (blockchain *BlockChain) processPortalCustodianDeposit(
	beaconHeight uint64, instructions []string, currentPortalState *CurrentPortalState) error {
	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}
	if len(instructions) !=  4 {
		return nil  // skip the instruction
	}
	db := blockchain.GetDatabase()

	// unmarshal instructions content
	var actionData metadata.PortalCustodianDepositContent
	err := json.Unmarshal([]byte(instructions[3]), &actionData)
	if err != nil {
		return err
	}

	depositStatus := instructions[2]
	if depositStatus == common.PortalCustodianDepositAcceptedChainStatus {
		keyCustodianState := lvdb.NewCustodianStateKey(beaconHeight, actionData.IncogAddressStr)
		// update custodian state
		if currentPortalState.CustodianPoolState[keyCustodianState] == nil {
			// new custodian
			newCustodian, err := NewCustodianState(actionData.IncogAddressStr, actionData.DepositedAmount, actionData.DepositedAmount, nil, nil, actionData.RemoteAddresses)
			if err != nil {
				return err
			}
			currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
		} else {
			// custodian deposited before
			// update state of the custodian
			custodian := currentPortalState.CustodianPoolState[keyCustodianState]
			totalCollateral := custodian.TotalCollateral + actionData.DepositedAmount
			freeCollateral := custodian.FreeCollateral + actionData.DepositedAmount
			holdingPubTokens := custodian.HoldingPubTokens
			lockedAmountCollateral := custodian.LockedAmountCollateral
			remoteAddresses := custodian.RemoteAddresses
			for tokenSymbol, address := range actionData.RemoteAddresses {
				if remoteAddresses[tokenSymbol] == "" {
					remoteAddresses[tokenSymbol] = address
				}
			}

			newCustodian, err := NewCustodianState(actionData.IncogAddressStr, totalCollateral, freeCollateral, holdingPubTokens, lockedAmountCollateral, remoteAddresses)
			if err != nil {
				return err
			}
			currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
		}

		// track custodian deposit into DB
		custodianDepositTrackKey := lvdb.NewCustodianDepositKey(actionData.TxReqID.String())
		custodianDepositTrackData := metadata.PortalCustodianDepositStatus{
			Status: common.PortalCustodianDepositAcceptedStatus,
			IncogAddressStr: actionData.IncogAddressStr,
			DepositedAmount : actionData.DepositedAmount,
		}

		custodianDepositDataBytes, _ := json.Marshal(custodianDepositTrackData)
		err = db.TrackCustodianDepositCollateral(
			[]byte(custodianDepositTrackKey),
			custodianDepositDataBytes,
		)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occured while tracking custodian deposit collateral: %+v", err)
			return nil
		}
	} else if depositStatus == common.PortalCustodianDepositRefundChainStatus {
		// track custodian deposit into DB
		custodianDepositTrackKey := lvdb.NewCustodianDepositKey(actionData.TxReqID.String())
		custodianDepositTrackData := metadata.PortalCustodianDepositStatus{
			Status: common.PortalCustodianDepositRefundStatus,
			IncogAddressStr: actionData.IncogAddressStr,
			DepositedAmount : actionData.DepositedAmount,
		}

		custodianDepositDataBytes, _ := json.Marshal(custodianDepositTrackData)
		err = db.TrackCustodianDepositCollateral(
			[]byte(custodianDepositTrackKey),
			custodianDepositDataBytes,
		)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occured while tracking custodian deposit collateral: %+v", err)
			return nil
		}
	}

	return nil
}

func (blockchain *BlockChain) processPortalUserRegister(
	beaconHeight uint64, instructions []string, currentPortalState *CurrentPortalState) error {
	db := blockchain.GetDatabase()

	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}

	if len(instructions) !=  4 {
		return nil  // skip the instruction
	}

	// parse instruction
	var portingRequestContent metadata.PortalPortingRequestContent
	err := json.Unmarshal([]byte(instructions[3]), &portingRequestContent)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshaling content string of porting request contribution instruction: %+v", err)
		return nil
	}

	reqStatus := instructions[2]

	switch reqStatus {
	case common.PortalPortingRequestAcceptedStatus:
		uniquePortingID := portingRequestContent.UniqueRegisterId
		txReqID := portingRequestContent.TxReqID
		tokenID := portingRequestContent.PTokenId

		porterAddress := portingRequestContent.IncogAddressStr
		amount := portingRequestContent.RegisterAmount

		custodiansDetail := portingRequestContent.Custodian
		portingFee := portingRequestContent.PortingFee

		// new request
		newPortingRequestState, err := NewPortingRequestState(
			uniquePortingID,
			txReqID,
			tokenID,
			porterAddress,
			amount,
			custodiansDetail,
			portingFee,
			common.PortalPortingReqWaitingStatus,
			beaconHeight + 1,
		)

		if err != nil {
			return err
		}

		//save transaction
		keyPortingRequestNewTxState := lvdb.NewPortingRequestTxKey(txReqID.String())
		err = db.StorePortingRequestItem([]byte(keyPortingRequestNewTxState), newPortingRequestState)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occurred while store porting request item: %+v", err)
			return nil
		}

		//save success porting request
		keyPortingRequestNewState := lvdb.NewPortingRequestKey(portingRequestContent.UniqueRegisterId)
		Logger.log.Infof("Porting request, save porting request with key %v", keyPortingRequestNewState)
		err = db.StorePortingRequestItem([]byte(keyPortingRequestNewState), newPortingRequestState)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occurred while store porting request item: %+v", err)
			return nil
		}

		//save custodian state
		for address, itemCustodian := range custodiansDetail {
			custodian := currentPortalState.CustodianPoolState[address]

			totalCollateral := custodian.TotalCollateral
			freeCollateral := custodian.FreeCollateral - itemCustodian.LockedAmountCollateral


			//update ptoken holded
			holdingPubTokensMapping := make(map[string]uint64)
			if custodian.HoldingPubTokens == nil {
				holdingPubTokensMapping[tokenID] =  itemCustodian.Amount
			} else {
				for ptokenId, value := range custodian.HoldingPubTokens {
					holdingPubTokensMapping[ptokenId] = value + itemCustodian.Amount
				}
			}
			holdingPubTokens := holdingPubTokensMapping

			//update collateral holded
			lockedAmountCollateralMapping := make(map[string]uint64)
			if custodian.LockedAmountCollateral == nil {
				lockedAmountCollateralMapping[tokenID] = itemCustodian.LockedAmountCollateral
			} else {
				for ptokenId, value := range custodian.LockedAmountCollateral {
					lockedAmountCollateralMapping[ptokenId] = value + itemCustodian.LockedAmountCollateral
				}
			}
			lockedAmountCollateral := lockedAmountCollateralMapping


			remoteAddresses := custodian.RemoteAddresses

			newCustodian, err := NewCustodianState(
				portingRequestContent.IncogAddressStr,
				totalCollateral,
				freeCollateral,
				holdingPubTokens,
				lockedAmountCollateral,
				remoteAddresses,
				)

			if err != nil {
				return err
			}
			Logger.log.Infof("Porting request, custodian key  %v", address)
			currentPortalState.CustodianPoolState[address] = newCustodian
		}

		//save waiting request porting state
		keyWaitingPortingRequest := lvdb.NewWaitingPortingReqKey(beaconHeight, portingRequestContent.UniqueRegisterId)
		Logger.log.Infof("Porting request, save waiting porting request with key %v", keyWaitingPortingRequest)
		currentPortalState.WaitingPortingRequests[keyWaitingPortingRequest] = newPortingRequestState

		break
	case common.PortalPortingRequestRejectedStatus:
		txReqID := portingRequestContent.TxReqID
		newPortingRequest := lvdb.PortingRequest{
			UniquePortingID: portingRequestContent.UniqueRegisterId,
			Amount:         portingRequestContent.RegisterAmount,
			TokenID:        portingRequestContent.PTokenId,
			PorterAddress:  portingRequestContent.IncogAddressStr,
			TxReqID:        txReqID,
			Status:			common.PortalPortingReqRejectedStatus,
			BeaconHeight:	beaconHeight + 1,
		}

		//save porting request
		keyPortingRequestNewState := lvdb.NewPortingRequestTxKey(txReqID.String())
		err = db.StorePortingRequestItem([]byte(keyPortingRequestNewState), newPortingRequest)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occurred while store porting request item: %+v", err)
			return nil
		}
		break
	}

	return nil
}

func (blockchain *BlockChain) processPortalUserReqPToken(
	beaconHeight uint64, instructions []string,
	currentPortalState *CurrentPortalState,
	updatingInfoByTokenID map[common.Hash]UpdatingInfo) error {
	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}

	if len(instructions) !=  4 {
		return nil  // skip the instruction
	}
	db := blockchain.GetDatabase()

	// unmarshal instructions content
	var actionData metadata.PortalRequestPTokensContent
	err := json.Unmarshal([]byte(instructions[3]), &actionData)
	if err != nil {
		Logger.log.Errorf("Can not unmarshal instruction content %v\n", err)
		return nil
	}

	reqStatus := instructions[2]
	if reqStatus == common.PortalReqPTokensAcceptedChainStatus {
		// remove portingRequest from waitingPortingRequests
		waitingPortingReqKey := lvdb.NewWaitingPortingReqKey(beaconHeight, actionData.UniquePortingID)
		isRemoved := removeWaitingPortingReqByKey(waitingPortingReqKey, currentPortalState)
		if !isRemoved {
			Logger.log.Errorf("Can not remove waiting porting request from portal state")
			return nil
		}

		// make sure user can not re-use proof for other portingID
		// update status of porting request with portingID
		err = db.UpdatePortingRequestStatus(actionData.UniquePortingID, common.PortalPortingReqSuccessStatus)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occurred while store porting request item status: %+v", err)
			return nil
		}

		// track reqPToken status by txID into DB
		reqPTokenTrackKey := lvdb.NewPortalReqPTokenKey(actionData.TxReqID.String())
		reqPTokenTrackData := metadata.PortalRequestPTokensStatus{
			Status: common.PortalReqPTokenAcceptedStatus,
			UniquePortingID: actionData.UniquePortingID,
			TokenID: actionData.TokenID,
			IncogAddressStr: actionData.IncogAddressStr,
			PortingAmount: actionData.PortingAmount,
			PortingProof: actionData.PortingProof,
		}
		reqPTokenTrackDataBytes, _ := json.Marshal(reqPTokenTrackData)
		err = db.TrackReqPTokens(
			[]byte(reqPTokenTrackKey),
			reqPTokenTrackDataBytes,
		)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occured while tracking request ptoken tx: %+v", err)
			return nil
		}

		// update bridge/portal token info
		incTokenID, err := common.Hash{}.NewHashFromStr(actionData.TokenID)
		if err != nil {
			Logger.log.Errorf("ERROR: Can not new hash from porting incTokenID: %+v", err)
			return nil
		}
		updatingInfo, found := updatingInfoByTokenID[*incTokenID]
		if found {
			updatingInfo.countUpAmt += actionData.PortingAmount
		} else {
			updatingInfo = UpdatingInfo{
				countUpAmt:      actionData.PortingAmount,
				deductAmt:       0,
				tokenID:         *incTokenID,
				externalTokenID: nil,
				isCentralized:   false,
			}
		}
		updatingInfoByTokenID[*incTokenID] = updatingInfo

	} else if reqStatus == common.PortalReqPTokensRejectedChainStatus {
		// track reqPToken and deposit proof into DB
		reqPTokenTrackKey := lvdb.NewPortalReqPTokenKey(actionData.TxReqID.String())
		reqPTokenTrackData := metadata.PortalRequestPTokensStatus{
			Status: common.PortalReqPTokenRejectedStatus,
			UniquePortingID: actionData.UniquePortingID,
		}
		reqPTokenTrackDataBytes, _ := json.Marshal(reqPTokenTrackData)
		err = db.TrackReqPTokens(
			[]byte(reqPTokenTrackKey),
			reqPTokenTrackDataBytes,
		)
		if err != nil {
			Logger.log.Errorf("ERROR: an error occured while tracking custodian deposit collateral: %+v", err)
			return nil
		}
	}

	return nil
}

func (blockchain *BlockChain) processPortalExchangeRates(beaconHeight uint64, instructions []string, currentPortalState *CurrentPortalState) error {
	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}

	db := blockchain.GetDatabase()

	// parse instruction
	var portingExchangeRatesContent metadata.PortalExchangeRatesContent
	err := json.Unmarshal([]byte(instructions[3]), &portingExchangeRatesContent)
	if err != nil {
		Logger.log.Errorf("ERROR: an error occurred while unmarshaling content string of portal exchange rates instruction: %+v", err)
		return nil
	}

	Logger.log.Infof("Portal exchange rates, data input %v", portingExchangeRatesContent)

	reqStatus := instructions[2]

	switch reqStatus {
	case common.PortalExchangeRatesSuccessStatus:
		//save db
		newExchangeRates, _ := NewExchangeRatesState(
			portingExchangeRatesContent.SenderAddress,
			portingExchangeRatesContent.Rates,
		)


		err = db.StoreExchangeRatesRequestItem([]byte(portingExchangeRatesContent.UniqueRequestId), newExchangeRates)

		if err != nil {
			Logger.log.Errorf("ERROR: Save exchange rates error: %+v", err)
			return err
		}

		currentPortalState.ExchangeRatesRequests[portingExchangeRatesContent.UniqueRequestId] = newExchangeRates

		Logger.log.Infof("Portal exchange rates, exchange rates request: count final exchange rate %v , exchange rate request %v", len(currentPortalState.FinalExchangeRates), len(currentPortalState.ExchangeRatesRequests))

	case common.PortalExchangeRatesRejectedStatus:
		//save db
		newExchangeRates := lvdb.ExchangeRatesRequest{
			SenderAddress: portingExchangeRatesContent.SenderAddress,
		}

		err = db.StoreExchangeRatesRequestItem([]byte(portingExchangeRatesContent.UniqueRequestId), newExchangeRates)

		if err != nil {
			Logger.log.Errorf("ERROR: Save exchange rates error: %+v", err)
			return err
		}

		break
	}

	return nil
}

func (blockchain *BlockChain) pickExchangesRatesFinal(beaconHeight uint64, currentPortalState *CurrentPortalState) error  {
	exchangeRatesKey := lvdb.NewFinalExchangeRatesKey(beaconHeight)

	Logger.log.Infof("Portal final exchange rates, pick final exchange rates from exchange rates, key %v, count final exchange rate %v , exchange rate request %v", exchangeRatesKey, len(currentPortalState.FinalExchangeRates), len(currentPortalState.ExchangeRatesRequests))

	//convert to slice
	var btcExchangeRatesSlice []uint64
	var bnbExchangeRatesSlice []uint64
	var prvExchangeRatesSlice []uint64
	for _, v := range currentPortalState.ExchangeRatesRequests {
		for key, rates := range v.Rates {
			switch key {
			case metadata.PortalTokenSymbolBTC:
				btcExchangeRatesSlice = append(btcExchangeRatesSlice, rates)
				break
			case metadata.PortalTokenSymbolBNB:
				bnbExchangeRatesSlice = append(bnbExchangeRatesSlice, rates)
				break
			case metadata.PortalTokenSymbolPRV:
				prvExchangeRatesSlice = append(prvExchangeRatesSlice, rates)
				break
			}
		}
	}

	//sort
	sort.SliceStable(btcExchangeRatesSlice, func(i, j int) bool {
		return btcExchangeRatesSlice[i] < btcExchangeRatesSlice[j]
	})

	sort.SliceStable(bnbExchangeRatesSlice, func(i, j int) bool {
		return bnbExchangeRatesSlice[i] < bnbExchangeRatesSlice[j]
	})

	sort.SliceStable(prvExchangeRatesSlice, func(i, j int) bool {
		return prvExchangeRatesSlice[i] < prvExchangeRatesSlice[j]
	})

	exchangeRatesList := make(map[string]lvdb.FinalExchangeRatesDetail)

	var btcAmount uint64
	var bnbAmount uint64
	var prvAmount uint64


	//get current value
	if len(btcExchangeRatesSlice) > 0 {
		btcAmount = calcMedian(btcExchangeRatesSlice)
	}

	if len(bnbExchangeRatesSlice) > 0 {
		bnbAmount = calcMedian(bnbExchangeRatesSlice)

	}

	if len(prvExchangeRatesSlice) > 0 {
		prvAmount = calcMedian(prvExchangeRatesSlice)
	}

	//if pre state exist
	if exchangeRatesState, ok := currentPortalState.FinalExchangeRates[exchangeRatesKey]; ok {
		Logger.log.Infof("Portal final exchange rates, pre block exits generate key %v", exchangeRatesKey)

		var btcAmountPreState uint64
		var bnbAmountPreState uint64
		var prvAmountPreState uint64
		if value, ok := exchangeRatesState.Rates[metadata.PortalTokenSymbolBTC]; ok {
			btcAmountPreState = value.Amount
		}

		if value, ok := exchangeRatesState.Rates[metadata.PortalTokenSymbolBNB]; ok {
			bnbAmountPreState = value.Amount
		}

		if value, ok := exchangeRatesState.Rates[metadata.PortalTokenSymbolPRV]; ok {
			prvAmountPreState = value.Amount
		}

		//pick current value and pre value state
		btcAmount = choicePrice(btcAmount, btcAmountPreState)
		bnbAmount = choicePrice(bnbAmount, bnbAmountPreState)
		prvAmount = choicePrice(prvAmount, prvAmountPreState)
	}

	//select
	if btcAmount > 0 {
		exchangeRatesList[metadata.PortalTokenSymbolBTC] = lvdb.FinalExchangeRatesDetail{
			Amount: btcAmount,
		}
	}

	if bnbAmount > 0 {
		exchangeRatesList[metadata.PortalTokenSymbolBNB] = lvdb.FinalExchangeRatesDetail{
			Amount: bnbAmount,
		}
	}

	if prvAmount > 0 {
		exchangeRatesList[metadata.PortalTokenSymbolPRV] = lvdb.FinalExchangeRatesDetail{
			Amount: prvAmount,
		}
	}


	if len(exchangeRatesList) > 0 {
		currentPortalState.FinalExchangeRates[exchangeRatesKey] = &lvdb.FinalExchangeRates{
			Rates: exchangeRatesList,
		}

		Logger.log.Infof("Portal final exchange rates, key %v, count final exchange rate %v", exchangeRatesKey, len(currentPortalState.FinalExchangeRates))
	}

	return nil
}

func calcMedian(ratesList []uint64) uint64 {
	mNumber := len(ratesList) / 2

	if len(ratesList) % 2 == 0 {
		return (ratesList[mNumber-1] + ratesList[mNumber]) / 2
	}

	return ratesList[mNumber]
}

func choicePrice(currentPrice uint64, prePrice uint64) uint64  {
	if currentPrice > 0 {
		return currentPrice
	} else {
		if prePrice > 0 {
			return prePrice
		}
	}

	return 0
}