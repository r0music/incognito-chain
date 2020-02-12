package blockchain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/incognitochain/incognito-chain/database"
	"github.com/incognitochain/incognito-chain/database/lvdb"
	"github.com/incognitochain/incognito-chain/metadata"
	"sort"
	"fmt"
	"github.com/binance-chain/go-sdk/types/msg"
	relaying "github.com/incognitochain/incognito-chain/relaying/bnb"
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
	for _, inst := range block.Body.Instructions {
		if len(inst) < 2 {
			continue // Not error, just not Portal instruction
		}
		var err error
		switch inst[0] {
		case strconv.Itoa(metadata.PortalCustodianDepositMeta):
			err = blockchain.processPortalCustodianDeposit(beaconHeight, inst, currentPortalState)
		case strconv.Itoa(metadata.PortalUserRegisterMeta):
			err = blockchain.processPortalUserRegister(beaconHeight, inst, currentPortalState)
		case strconv.Itoa(metadata.PortalUserRequestPTokenMeta):
			err = blockchain.processPortalUserReqPToken(beaconHeight, inst, currentPortalState)
		}

		if err != nil {
			Logger.log.Error(err)
			return nil
		}
	}

	//todo: check timeout register porting via beacon height
	// all request timeout ? unhold

	// store updated currentPortalState to leveldb with new beacon height
	err = storePortalStateToDB(db, beaconHeight+1, currentPortalState)
	if err != nil {
		Logger.log.Error(err)
	}
	return nil
}

// todo
func (blockchain *BlockChain) processPortalCustodianDeposit(
	beaconHeight uint64, instructions []string, currentPortalState *CurrentPortalState) error {
	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}

	// parse instruction
	actionContentB64Str := instructions[1]
	actionContentBytes, err := base64.StdEncoding.DecodeString(actionContentB64Str)
	if err != nil {
		return err
	}
	var actionData metadata.PortalCustodianDepositAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		return err
	}

	meta := actionData.Meta
	keyCustodianState := lvdb.NewCustodianStateKey(beaconHeight, meta.IncogAddressStr)

	if currentPortalState.CustodianPoolState[keyCustodianState] == nil {
		// new custodian
		newCustodian, err := NewCustodianState(meta.IncogAddressStr, meta.DepositedAmount, meta.DepositedAmount, nil, meta.RemoteAddresses)
		if err != nil {
			return err
		}
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	} else {
		// custodian deposited before
		// update state of the custodian
		custodian := currentPortalState.CustodianPoolState[meta.IncogAddressStr]
		totalCollateral := custodian.TotalCollateral + meta.DepositedAmount
		freeCollateral := custodian.FreeCollateral + meta.DepositedAmount
		holdingPubTokens := custodian.HoldingPubTokens
		remoteAddresses := custodian.RemoteAddresses
		for tokenSymbol, address := range meta.RemoteAddresses {
			if remoteAddresses[tokenSymbol] == "" {
				remoteAddresses[tokenSymbol] = address
			}
		}

		newCustodian, err := NewCustodianState(meta.IncogAddressStr, totalCollateral, freeCollateral, holdingPubTokens, remoteAddresses)
		if err != nil {
			return err
		}
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	}

	return nil
}

func (blockchain *BlockChain) processPortalUserRegister(
	beaconHeight uint64, instructions []string, currentPortalState *CurrentPortalState) error {
	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}

	// parse instruction
	actionContentB64Str := instructions[1]
	actionContentBytes, err := base64.StdEncoding.DecodeString(actionContentB64Str)
	if err != nil {
		return err
	}
	var actionData metadata.PortalUserRegisterAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		return err
	}


	meta := actionData.Meta
	keyPortingRequestState := lvdb.NewPortingRequestKey(beaconHeight, meta.UniqueRegisterId)

	if currentPortalState.PortingRequests[keyPortingRequestState] != nil {
		Logger.log.Errorf("Unique porting id is duplicated")
		return nil
	}

	//find custodian
	//todo: get exchangeRate via tokenid
	pickCustodian, err := pickCustodian(meta.RegisterAmount, 1, meta.PTokenId, currentPortalState.CustodianPoolState)

	if err != nil {
		return err
	}

	uniquePortingID := meta.UniqueRegisterId
	txReqID := actionData.TxReqID
	tokenID := meta.PTokenId

	porterAddress := meta.IncogAddressStr
	amount := meta.RegisterAmount

	custodians := pickCustodian
	portingFee := meta.PortingFee

	// new request
	newPortingRequestState, err := NewPortingRequestState(
		uniquePortingID,
		txReqID,
		tokenID,
		porterAddress,
		amount,
		custodians,
		portingFee,
		)

	if err != nil {
		return err
	}

	currentPortalState.PortingRequests[keyPortingRequestState] = newPortingRequestState
	//todo: lock custodian
/*
	currentPortalState.CustodianPoolState[pickCustodian.] == nil {
		// new custodian
		newCustodian, err := NewCustodianState(meta.IncogAddressStr, meta.DepositedAmount, meta.DepositedAmount, nil, meta.RemoteAddresses)
		if err != nil {
			return err
		}
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	} else {
		// custodian deposited before
		// update state of the custodian
		custodian := currentPortalState.CustodianPoolState[meta.IncogAddressStr]
		totalCollateral := custodian.TotalCollateral + meta.DepositedAmount
		freeCollateral := custodian.FreeCollateral + meta.DepositedAmount
		holdingPubTokens := custodian.HoldingPubTokens
		remoteAddresses := custodian.RemoteAddresses
		for tokenSymbol, address := range meta.RemoteAddresses {
			if remoteAddresses[tokenSymbol] == "" {
				remoteAddresses[tokenSymbol] = address
			}
		}

		newCustodian, err := NewCustodianState(meta.IncogAddressStr, totalCollateral, freeCollateral, holdingPubTokens, remoteAddresses)
		if err != nil {
			return err
		}
		currentPortalState.CustodianPoolState[keyCustodianState] = newCustodian
	}
*/
	return nil
}

func pickCustodian(amount uint64, exchangeRate uint64, tokenId string, custodianState map[string]*lvdb.CustodianState) (map[string]lvdb.MatchingCustodianDetail, error) {

	type custodianStateSlice struct {
		Key   string
		Value *lvdb.CustodianState
	}

	var sortCustodianStateByFreeCollateral []custodianStateSlice
	for k, v := range custodianState {
		_, tokenIdExist := v.RemoteAddresses[tokenId]
		if !tokenIdExist {
			continue
		}

		sortCustodianStateByFreeCollateral = append(sortCustodianStateByFreeCollateral, custodianStateSlice{k, v})
	}

	sort.Slice(sortCustodianStateByFreeCollateral, func(i, j int) bool {
		return sortCustodianStateByFreeCollateral[i].Value.FreeCollateral <= sortCustodianStateByFreeCollateral[j].Value.FreeCollateral
	})

	//pick custodian
	for _, kv := range sortCustodianStateByFreeCollateral {
		if kv.Value.FreeCollateral >= (amount * 1.5) * exchangeRate {
			result := make(map[string]lvdb.MatchingCustodianDetail)
			result[kv.Key] = lvdb.MatchingCustodianDetail{
				RemoteAddress: tokenId,
				Amount: amount*exchangeRate,
			}

			return result, nil
		}
	}

	return map[string]lvdb.MatchingCustodianDetail{}, errors.New("Can not pickup custodian")
}

func (blockchain *BlockChain) processPortalUserReqPToken(
	beaconHeight uint64, instructions []string, currentPortalState *CurrentPortalState) error {
	if currentPortalState == nil {
		Logger.log.Errorf("current portal state is nil")
		return nil
	}

	// parse instruction
	actionContentB64Str := instructions[1]
	actionContentBytes, err := base64.StdEncoding.DecodeString(actionContentB64Str)
	if err != nil {
		return err
	}
	var actionData metadata.PortalRequestPTokensAction
	err = json.Unmarshal(actionContentBytes, &actionData)
	if err != nil {
		return err
	}

	meta := actionData.Meta
	// check meta.UniquePortingID is in PortingRequests list in portal state or not
	portingID := meta.UniquePortingID
	keyWaitingPortingRequest := lvdb.NewPortingReqKey(beaconHeight, portingID)
	waitingPortingRequest := currentPortalState.PortingRequests[keyWaitingPortingRequest]

	if waitingPortingRequest == nil {
		return errors.New("PortingID is not existed in waiting porting requests list")
	}

	// check tokenID
	if meta.TokenID != waitingPortingRequest.TokenID {
		return errors.New("TokenID is not correct in portingID req")
	}

	// check porting amount
	if meta.PortingAmount != waitingPortingRequest.Amount {
		return errors.New("PortingAmount is not correct in portingID req")
	}

	if meta.TokenID == "BTC" {
		//todo:
	} else if meta.TokenID == "BNB" {
		// parse txproof in meta
		txProofBNB, err := relaying.ParseProofFromB64EncodeJsonStr(meta.PortingProof)
		if err != nil {
			return errors.New("PortingProof is invalid")
		}

		// parse Tx from Data in txProofBNB
		txBNB, err := relaying.ParseTxFromData(txProofBNB.Data)
		if err != nil {
			return errors.New("Data in PortingProof is invalid")
		}

		// check whether amount transfer in txBNB is equal porting amount or not
		// check receiver and amount in tx
		// get list matching custodians in waitingPortingRequest
		custodians := waitingPortingRequest.Custodians
		outputs := txBNB.Msgs[0].(msg.SendMsg).Outputs

		for _, cusDetail := range custodians {
			remoteAddressNeedToBeTransfer := cusDetail.RemoteAddress
			amountNeedToBeTransfer := cusDetail.Amount

			for _, out := range outputs {
				addr := string(out.Address)
				if addr != remoteAddressNeedToBeTransfer {
					continue
				}

				// calculate amount that was transferred to custodian's remote address
				amountTransfer := int64(0)
				for _, coin := range out.Coins {
					if coin.Denom == relaying.DenomBNB {
						amountTransfer += coin.Amount
					}
				}

				if amountTransfer != int64(amountNeedToBeTransfer) {
					return fmt.Errorf("TxProof-BNB is invalid - Amount transfer to %s must be equal %d, but got %d",
						addr, amountNeedToBeTransfer, amountTransfer)
				}
			}

		}
	} else {
		return errors.New("TokenID is not supported currently on Portal")
	}

	// create instruction mint ptoken to IncogAddressStr and send to shard
	//todo:


	return nil
}