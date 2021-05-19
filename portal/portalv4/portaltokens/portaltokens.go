package portaltokens

import (
	"encoding/base64"
	"encoding/json"

	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/dataaccessobject/statedb"
	"github.com/incognitochain/incognito-chain/metadata"
)

type PortalTokenProcessor interface {
	IsValidRemoteAddress(address string, bcr metadata.ChainRetriever) (bool, error)
	GetChainID() string
	GetMinTokenAmount() uint64
	GetMultipleTokenAmount() uint64
	ConvertExternalToIncAmount(incAmt uint64) uint64
	ConvertIncToExternalAmount(incAmt uint64) uint64
	GetTxHashFromRawTx(rawTx string) (string, error)

	ParseAndVerifyShieldProof(
		proof string, bc metadata.ChainRetriever, expectedReceivedMultisigAddress string, chainCodeSeed string) (bool, []*statedb.UTXO, error)
	ParseAndVerifyUnshieldProof(
		proof string, bc metadata.ChainRetriever, expectedReceivedMultisigAddress string, chainCodeSeed string, expectPaymentInfo []*OutputTx, utxos []*statedb.UTXO) (bool, []*statedb.UTXO, string, uint64, error)
	MatchUTXOsAndUnshieldIDs(utxos map[string]*statedb.UTXO, waitingUnshieldReqs map[string]*statedb.WaitingUnshieldRequest, tinyAmount uint64) []*BroadcastTx

	CreateRawExternalTx(inputs []*statedb.UTXO, outputs []*OutputTx, networkFee uint64, bc metadata.ChainRetriever) (string, string, error)
	PartSignOnRawExternalTx(seedKey []byte, masterPubKeys [][]byte, numSigsRequired int, rawTxBytes []byte, inputs []*statedb.UTXO) ([][]byte, string, error)
	GenerateOTMultisigAddress(masterPubKeys [][]byte, numSigsRequired int, chainCodeSeed string) ([]byte, string, error)
}

type PortalToken struct {
	ChainID             string
	MinTokenAmount      uint64 // set MinTokenAmount to avoid attacking with amount is less than smallest unit of cryptocurrency, such as satoshi in BTC
	MultipleTokenAmount uint64 // amount token must be a multiple of this param in order to avoid not consistent when converting between public token and private token
	ExternalInputSize   uint   // they are used to estimate size of external txs (in byte)
	ExternalOutputSize  uint
	ExternalTxMaxSize   uint
}

type BroadcastTx struct {
	UTXOs       []*statedb.UTXO
	UnshieldIDs []string
}

type OutputTx struct {
	ReceiverAddress string
	Amount          uint64
}

func (p PortalToken) GetExpectedMemoForShielding(incAddress string) string {
	type shieldingMemoStruct struct {
		IncAddress string `json:"ShieldingIncAddress"`
	}
	memoShielding := shieldingMemoStruct{IncAddress: incAddress}
	memoShieldingBytes, _ := json.Marshal(memoShielding)
	memoShieldingHashBytes := common.HashB(memoShieldingBytes)
	memoShieldingStr := base64.StdEncoding.EncodeToString(memoShieldingHashBytes)
	return memoShieldingStr
}

func (p PortalToken) GetExpectedMemoForRedeem(redeemID string, custodianAddress string) string {
	type redeemMemoStruct struct {
		RedeemID                  string `json:"RedeemID"`
		CustodianIncognitoAddress string `json:"CustodianIncognitoAddress"`
	}

	redeemMemo := redeemMemoStruct{
		RedeemID:                  redeemID,
		CustodianIncognitoAddress: custodianAddress,
	}
	redeemMemoBytes, _ := json.Marshal(redeemMemo)
	redeemMemoHashBytes := common.HashB(redeemMemoBytes)
	redeemMemoStr := base64.StdEncoding.EncodeToString(redeemMemoHashBytes)
	return redeemMemoStr
}
