package metadata

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/incognitochain/incognito-chain/privacy/coin"
	"strconv"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
)

type InitPTokenResponse struct {
	MetadataBase
	RequestedTxID common.Hash
}

func NewInitPTokenResponse(requestedTxID common.Hash, metaType int) *InitPTokenResponse {
	metadataBase := MetadataBase{
		Type: metaType,
	}
	return &InitPTokenResponse{
		RequestedTxID: requestedTxID,
		MetadataBase:  metadataBase,
	}
}

func (iRes InitPTokenResponse) CheckTransactionFee(tr Transaction, minFee uint64, beaconHeight int64, db *statedb.StateDB) bool {
	// no need to have fee for this tx
	return true
}

func (iRes InitPTokenResponse) ValidateTxWithBlockChain(tx Transaction, chainRetriever ChainRetriever, shardViewRetriever ShardViewRetriever, beaconViewRetriever BeaconViewRetriever, shardID byte, transactionStateDB *statedb.StateDB) (bool, error) {
	// no need to validate tx with blockchain, just need to validate with requested tx (via RequestedTxID) in current block
	return true, nil
}

//ValidateSanityData performs the following verification:
//	1. Check transaction type
func (iRes InitPTokenResponse) ValidateSanityData(chainRetriever ChainRetriever, shardViewRetriever ShardViewRetriever, beaconViewRetriever BeaconViewRetriever, beaconHeight uint64, tx Transaction) (bool, bool, error) {
	//Step 1
	if tx.GetType() != common.TxCustomTokenPrivacyType {
		return false, false, NewMetadataTxError(InitPTokenResponseValidateSanityDataError, fmt.Errorf("tx InitPTokenResponse must have type `%v`", common.TxCustomTokenPrivacyType))
	}

	return false, true, nil
}

func (iRes InitPTokenResponse) ValidateMetadataByItself() bool {
	// The validation just need to check at tx level, so returning true here
	return iRes.Type == InitPTokenResponseMeta
}

func (iRes InitPTokenResponse) Hash() *common.Hash {
	record := iRes.RequestedTxID.String()
	record += iRes.MetadataBase.Hash().String()

	// final hash
	hash := common.HashH([]byte(record))
	return &hash
}

func (iRes *InitPTokenResponse) CalculateSize() uint64 {
	return calculateSize(iRes)
}

//VerifyMinerCreatedTxBeforeGettingInBlock validates if the response is a reply to an instruction from the beacon chain.
//The response is valid for a specific instruction if
//	1. the instruction has a valid metadata type
//	2. the requested txIDs match
//	3. the minted public key and the one in the instruction match
//	4. the minted tx random and the one in the instruction match
//	5. the minted amount and the requested amount match
//	6. the minted and requested tokens match
//It returns false if no instruction from the beacon satisfies the above conditions.
//
//TODO: reviewers should double-check if the above conditions are sufficient
func (iRes InitPTokenResponse) VerifyMinerCreatedTxBeforeGettingInBlock(mintData *MintData,
	shardID byte,
	tx Transaction,
	chainRetriever ChainRetriever,
	ac *AccumulatedValues,
	shardViewRetriever ShardViewRetriever,
	beaconViewRetriever BeaconViewRetriever,
) (bool, error) {
	idx := -1
	Logger.log.Infof("Number of instructions: %v\n", len(mintData.Insts))
	for i, inst := range mintData.Insts {
		if len(inst) < 4 { // this is not InitPTokenRequest instruction
			continue
		}

		Logger.log.Infof("Currently processing instruction: %v\n", inst)

		instMetaType := inst[0]
		if mintData.InstsUsed[i] > 0 ||
			instMetaType != strconv.Itoa(InitPTokenRequestMeta) {
			continue
		}

		contentBytes, err := base64.StdEncoding.DecodeString(inst[3])
		if err != nil {
			Logger.log.Errorf("WARNING - VALIDATION: an error occurred while parsing instruction content: %v\n", err)
			continue
		}
		var acceptedInst InitPTokenAcceptedInst
		err = json.Unmarshal(contentBytes, &acceptedInst)
		if err != nil {
			Logger.log.Error("WARNING - VALIDATION: an error occured while parsing instruction content: ", err)
			continue
		}

		recvPubKey, txRandom, err := coin.ParseOTAInfoFromString(acceptedInst.OTAStr, acceptedInst.TxRandomStr)
		if err != nil {
			Logger.log.Errorf("WARNING - VALIDATION: cannot parse OTA params (%v, %v): %v", acceptedInst.OTAStr, acceptedInst.TxRandomStr, err)
			continue
		}

		if iRes.RequestedTxID.String() != acceptedInst.RequestedTxID.String() {
			Logger.log.Infof("txHash mismatch: %v != %v\n", iRes.RequestedTxID.String(), acceptedInst.RequestedTxID.String())
			continue
		}

		_, mintedCoin, mintedTokenID, err := tx.GetTxMintData()
		if err != nil {
			return false, fmt.Errorf("cannot get tx minted data of txResp %v: %v", tx.Hash().String(), err)
		}

		if !bytes.Equal(mintedCoin.GetPublicKey().ToBytesS(), recvPubKey.ToBytesS()){
			Logger.log.Infof("public keys mismatch: %v != %v\n", mintedCoin.GetPublicKey().ToBytesS(), recvPubKey.ToBytesS())
			continue
		}

		if !bytes.Equal(mintedCoin.GetTxRandom().Bytes(), txRandom.Bytes()) {
			Logger.log.Infof("txRandoms mismatch: %v != %v\n", mintedCoin.GetTxRandom().Bytes(), txRandom.Bytes())
			continue
		}

		if mintedCoin.GetValue() != acceptedInst.Amount {
			Logger.log.Infof("amounts mismatch: %v != %v\n", mintedCoin.GetValue(), acceptedInst.Amount)
			continue
		}

		if mintedTokenID.String() != acceptedInst.TokenID.String() {
			Logger.log.Infof("tokenID mismatch: %v != %v\n", mintedTokenID.String(), acceptedInst.TokenID.String())
			continue
		}

		idx = i
		break
	}

	if idx == -1 { // not found the issuance request tx for this response
		return false, fmt.Errorf("no InitPTokenRequest tx found for tx %s", tx.Hash().String())
	}
	mintData.InstsUsed[idx] = 1

	return true, nil
}
