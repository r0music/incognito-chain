package transaction

import (
	"github.com/incognitochain/incognito-chain/common"
	"github.com/incognitochain/incognito-chain/privacy"
	"github.com/incognitochain/incognito-chain/wallet"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInitTxPrivacyToken(t *testing.T) {
	senderKey, err := wallet.Base58CheckDeserialize("112t8rnXCqbbNYBquntyd6EvDT4WiDDQw84ZSRDKmazkqrzi6w8rWyCVt7QEZgAiYAV4vhJiX7V9MCfuj4hGLoDN7wdU1LoWGEFpLs59X7K3")
	assert.Equal(t, nil, err)
	err = senderKey.KeySet.InitFromPrivateKey(&senderKey.KeySet.PrivateKey)
	assert.Equal(t, nil, err)
	//senderPaymentAddress := senderKey.KeySet.PaymentAddress
	//senderPublicKey := senderPaymentAddress.Pk

	//senderPaymentAddrStr := senderKey.Base58CheckSerialize(wallet.PaymentAddressType)

	shardID := common.GetShardIDFromLastByte(senderKey.KeySet.PaymentAddress.Pk[len(senderKey.KeySet.PaymentAddress.Pk)-1])

	// receiver's address
	receiverPaymentAddress, _ := wallet.Base58CheckDeserialize("1Uv3BkYiWy9Mjt1yBa4dXBYKo3az22TeCVEpeXN93ieJ8qhrTDuUZBzsPZWjjP2AeRQnjw1y18iFPHTRuAqqufwVC1vNUAWs4wHFbbWC2")
	initAmount := uint64(10000)
	paymentInfo := []*privacy.PaymentInfo{{PaymentAddress: senderKey.KeySet.PaymentAddress, Amount: initAmount}}

	inputCoinsPRV := []*privacy.InputCoin{}
	paymentInfoPRV := []*privacy.PaymentInfo{}

	// token param for init new token
	tokenParam := &CustomTokenPrivacyParamTx{
		PropertyID:     "",
		PropertyName:   "Token 1",
		PropertySymbol: "Token 1",
		Amount:         initAmount,
		TokenTxType:    CustomTokenInit,
		Receiver:       paymentInfo,
		TokenInput:     []*privacy.InputCoin{},
		Mintable:       false,
		Fee:            0,
	}

	hasPrivacyForPRV := false
	hasPrivacyForToken := false

	paramToCreateTx := NewTxPrivacyTokenInitParams(&senderKey.KeySet.PrivateKey,
		paymentInfoPRV, inputCoinsPRV, 0, tokenParam, db, nil,
		hasPrivacyForPRV, hasPrivacyForToken, shardID, []byte{})

	// init tx
	tx := new(TxCustomTokenPrivacy)
	err = tx.Init(paramToCreateTx)
	assert.Equal(t, nil, err)

	// convert to JSON string and revert
	txJsonString := tx.JSONString()
	txHash := tx.Hash()

	tx1 := new(TxCustomTokenPrivacy)
	tx1.UnmarshalJSON([]byte(txJsonString))
	txHash1 := tx1.Hash()
	assert.Equal(t, txHash, txHash1)

	// get actual tx size
	txActualSize := tx.GetTxActualSize()
	assert.Greater(t, txActualSize, uint64(0))

	txPrivacyTokenActualSize := tx.GetTxPrivacyTokenActualSize()
	assert.Greater(t, txPrivacyTokenActualSize, uint64(0))

	isValidFee := tx.CheckTransactionFee(uint64(0))
	assert.Equal(t, true, isValidFee)

	isValidFeeToken := tx.CheckTransactionFeeByFeeToken(uint64(0))
	assert.Equal(t, true, isValidFeeToken)

	isValidFeeTokenForTokenData := tx.CheckTransactionFeeByFeeTokenForTokenData(uint64(0))
	assert.Equal(t, true, isValidFeeTokenForTokenData)

	isValidType := tx.ValidateType()
	assert.Equal(t, true, isValidType)

	//err = tx.ValidateTxWithCurrentMempool(nil)
	//assert.Equal(t, nil, err)

	err = tx.ValidateTxWithBlockChain(nil, shardID, db)
	assert.Equal(t, nil, err)

	isValidSanity, err := tx.ValidateSanityData(nil)
	assert.Equal(t, true, isValidSanity)
	assert.Equal(t, nil, err)

	isValidTxItself, err := tx.ValidateTxByItself(hasPrivacyForPRV, db, nil, shardID)
	assert.Equal(t, true, isValidTxItself)
	assert.Equal(t, nil, err)

	//isValidTx, err := tx.ValidateTransaction(hasPrivacyForPRV, db, shardID, tx.GetTokenID())
	//fmt.Printf("Err: %v\n", err)
	//assert.Equal(t, true, isValidTx)
	//assert.Equal(t, nil, err)

	_ = tx.GetProof()
	//assert.Equal(t, nil, proof)

	pubKeyReceivers, amounts := tx.GetTokenReceivers()
	assert.Equal(t, 1, len(pubKeyReceivers))
	assert.Equal(t, 1, len(amounts))
	assert.Equal(t, initAmount, amounts[0])

	isUniqueReceiver, uniquePubKey, uniqueAmount, tokenID := tx.GetTransferData()
	assert.Equal(t, true, isUniqueReceiver)
	assert.Equal(t, initAmount, uniqueAmount)
	assert.Equal(t, tx.GetTokenID(), tokenID)
	receiverPubKeyBytes := make([]byte, common.PublicKeySize)
	copy(receiverPubKeyBytes, senderKey.KeySet.PaymentAddress.Pk)
	assert.Equal(t, uniquePubKey, receiverPubKeyBytes)

	isCoinBurningTx := tx.IsCoinsBurning()
	assert.Equal(t, false, isCoinBurningTx)

	txValue := tx.CalculateTxValue()
	assert.Equal(t, initAmount, txValue)

	listSerialNumber := tx.ListSerialNumbersHashH()
	assert.Equal(t, 0, len(listSerialNumber))

	sigPubKey := tx.GetSigPubKey()
	assert.Equal(t, common.SigPubKeySize, len(sigPubKey))

	// store init tx

	// get output coin token from tx
	outputCoins := ConvertOutputCoinToInputCoin(tx.TxPrivacyTokenData.TxNormal.Proof.GetOutputCoins())

	// calculate serial number for input coins
	serialNumber := new(privacy.Point).Derive(privacy.PedCom.G[privacy.PedersenPrivateKeyIndex],
		new(privacy.Scalar).FromBytesS(senderKey.KeySet.PrivateKey),
		outputCoins[0].CoinDetails.GetSNDerivator())
	outputCoins[0].CoinDetails.SetSerialNumber(serialNumber)

	db.StorePrivacyToken(*tx.GetTokenID(), tx.Hash()[:])
	db.StoreCommitments(*tx.GetTokenID(), senderKey.KeySet.PaymentAddress.Pk[:], [][]byte{outputCoins[0].CoinDetails.GetCoinCommitment().ToBytesS()}, shardID)

	listTokens, err := db.ListPrivacyToken()
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(listTokens))

	transferAmount := uint64(10)

	paymentInfo2 := []*privacy.PaymentInfo{{PaymentAddress: receiverPaymentAddress.KeySet.PaymentAddress, Amount: transferAmount}}

	// token param for transfer token
	tokenParam2 := &CustomTokenPrivacyParamTx{
		PropertyID:     tx.GetTokenID().String(),
		PropertyName:   "Token 1",
		PropertySymbol: "Token 1",
		Amount:         transferAmount,
		TokenTxType:    CustomTokenTransfer,
		Receiver:       paymentInfo2,
		TokenInput:     outputCoins,
		Mintable:       false,
		Fee:            0,
	}

	paramToCreateTx2 := NewTxPrivacyTokenInitParams(&senderKey.KeySet.PrivateKey,
		paymentInfoPRV, inputCoinsPRV, 0, tokenParam2, db, nil,
		hasPrivacyForPRV, true, shardID, []byte{})

	// init tx
	tx2 := new(TxCustomTokenPrivacy)
	err = tx2.Init(paramToCreateTx2)
	assert.Equal(t, nil, err)

	err = tx2.ValidateTxWithBlockChain(nil, shardID, db)
	assert.Equal(t, nil, err)

	isValidSanity, err = tx2.ValidateSanityData(nil)
	assert.Equal(t, true, isValidSanity)
	assert.Equal(t, nil, err)

	isValidTxItself, err = tx2.ValidateTxByItself(hasPrivacyForPRV, db, nil, shardID)
	assert.Equal(t, true, isValidTxItself)
	assert.Equal(t, nil, err)

	txValue2 := tx2.CalculateTxValue()
	assert.Equal(t, uint64(0), txValue2)
}
