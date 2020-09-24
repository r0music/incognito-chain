package blsbftv2

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/common/base58"
	"github.com/incognitochain/incognito-chain/consensus/signatureschemes/blsmultisig"
	"github.com/incognitochain/incognito-chain/consensus/signatureschemes/bridgesig"
	"github.com/incognitochain/incognito-chain/incognitokey"
)

type blockValidation interface {
	common.BlockInterface
	AddValidationField(validationData string) error
}

type ValidationData struct {
	ProducerBLSSig []byte
	ProducerBriSig []byte
	ValidatiorsIdx []int
	AggSig         []byte
	BridgeSig      [][]byte
}

func DecodeValidationData(data string) (*ValidationData, error) {
	var valData ValidationData
	err := json.Unmarshal([]byte(data), &valData)
	if err != nil {
		return nil, NewConsensusError(DecodeValidationDataError, err)
	}
	return &valData, nil
}

func EncodeValidationData(validationData ValidationData) (string, error) {
	result, err := json.Marshal(validationData)
	if err != nil {
		return "", NewConsensusError(EncodeValidationDataError, err)
	}
	return string(result), nil
}

func (e BLSBFT_V2) CreateValidationData(block common.BlockInterface) ValidationData {
	var valData ValidationData
	valData.ProducerBLSSig, _ = e.UserKeySet.BriSignData(block.Hash().GetBytes())
	return valData
}

func ValidateProducerSig(block common.BlockInterface) error {
	valData, err := DecodeValidationData(block.GetValidationField())
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}

	producerKey := incognitokey.CommitteePublicKey{}
	err = producerKey.FromBase58(block.GetProposer())
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	//start := time.Now()
	if err := validateSingleBriSig(block.Hash(), valData.ProducerBLSSig, producerKey.MiningPubKey[common.BridgeConsensus]); err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	//end := time.Now().Sub(start)
	//fmt.Printf("ConsLog just verify %v\n", end.Seconds())
	return nil
}

func ValidateCommitteeSig(block common.BlockInterface, committee []incognitokey.CommitteePublicKey) error {
	//@tin Check here
	valData, err := DecodeValidationData(block.GetValidationField())
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	committeeBLSKeys := []blsmultisig.PublicKey{}
	for _, member := range committee {
		committeeBLSKeys = append(committeeBLSKeys, member.MiningPubKey[common.BlsConsensus])
	}
	if err := validateBLSSig(block.Hash(), valData.AggSig, valData.ValidatiorsIdx, committeeBLSKeys); err != nil {
		committeeStr, _ := incognitokey.CommitteeKeyListToString(committee)
		fmt.Printf("[ValidateBLS] Validate BLS sig of block %v return error %v; Validators index %v; Signature %v; committee %v\n", block.Hash().String(), err, valData.ValidatiorsIdx, valData.AggSig, committeeStr)
		return NewConsensusError(UnExpectedError, err)
	}
	return nil
}

func (e BLSBFT_V2) ValidateData(data []byte, sig string, publicKey string) error {
	sigByte, _, err := base58.Base58Check{}.Decode(sig)
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	publicKeyByte := []byte(publicKey)
	// if err != nil {
	// 	return NewConsensusError(UnExpectedError, err)
	// }
	//fmt.Printf("ValidateData data %v, sig %v, publicKey %v\n", data, sig, publicKeyByte)
	dataHash := new(common.Hash)
	dataHash.NewHash(data)
	_, err = bridgesig.Verify(publicKeyByte, dataHash.GetBytes(), sigByte) //blsmultisig.Verify(sigByte, data, []int{0}, []blsmultisig.PublicKey{publicKeyByte})
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	return nil
}

func validateSingleBLSSig(
	dataHash *common.Hash,
	blsSig []byte,
	selfIdx int,
	committee []blsmultisig.PublicKey,
) error {
	//start := time.Now()
	result, err := blsmultisig.Verify(blsSig, dataHash.GetBytes(), []int{selfIdx}, committee)
	//end := time.Now().Sub(start)
	//fmt.Printf("ConsLog single verify %v\n", end.Seconds())
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	if !result {
		return NewConsensusError(UnExpectedError, errors.New("invalid BLS Signature"))
	}
	return nil
}

func validateSingleBriSig(
	dataHash *common.Hash,
	briSig []byte,
	validatorPk []byte,
) error {
	result, err := bridgesig.Verify(validatorPk, dataHash.GetBytes(), briSig)
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	if !result {
		return NewConsensusError(UnExpectedError, errors.New("invalid BRI Signature"))
	}
	return nil
}

func validateBLSSig(
	dataHash *common.Hash,
	aggSig []byte,
	validatorIdx []int,
	committee []blsmultisig.PublicKey,
) error {
	result, err := blsmultisig.Verify(aggSig, dataHash.GetBytes(), validatorIdx, committee)
	if err != nil {
		return NewConsensusError(UnExpectedError, err)
	}
	if !result {
		return NewConsensusError(UnExpectedError, errors.New("Invalid Signature!"))
	}
	return nil
}

func (e BLSBFT_V2) ValidateBlockWithConsensus(block common.BlockInterface) error {

	return nil
}
