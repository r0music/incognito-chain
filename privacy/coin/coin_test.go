package coin

import (
	"github.com/incognitochain/incognito-chain/incognitokey"
	"github.com/stretchr/testify/assert"

	"github.com/incognitochain/incognito-chain/privacy/key"
	"testing"
)

func TestIsCoinBelong(t *testing.T) {
	privateKey0 := key.GeneratePrivateKey([]byte{0})
	keyset0 := new(incognitokey.KeySet)
	err := keyset0.InitFromPrivateKey(&privateKey0)
	assert.Equal(t, nil, err)

	privateKey1 := key.GeneratePrivateKey([]byte{1})
	keyset1 := new(incognitokey.KeySet)
	err = keyset1.InitFromPrivateKey(&privateKey1)
	assert.Equal(t, nil, err)

	c0, err := createNewCoin(10, keyset0.PaymentAddress, 0, 0)
	assert.Equal(t, nil, err)
	assert.Equal(t, false, c0.IsEncrypted())
	c0.ConcealData(keyset0.PaymentAddress.GetPublicView())

	c1, err := createNewCoin(10, keyset1.PaymentAddress, 0, 0)
	assert.Equal(t, nil, err)
	assert.Equal(t, false, c1.IsEncrypted())
	c1.ConcealData(keyset1.PaymentAddress.GetPublicView())

	assert.Equal(t, true, IsCoinBelongToViewKey(c0, keyset0.ReadonlyKey))
	assert.Equal(t, true, IsCoinBelongToViewKey(c1, keyset1.ReadonlyKey))
	assert.Equal(t, false, IsCoinBelongToViewKey(c0, keyset1.ReadonlyKey))
	assert.Equal(t, false, IsCoinBelongToViewKey(c1, keyset0.ReadonlyKey))
}