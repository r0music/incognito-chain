package zkp

import (
	"encoding/binary"
	"fmt"
	"github.com/pkg/errors"
	"math/big"
	"time"

	"github.com/ninjadotorg/constant/privacy"
)

type OneOutOfManyStatement struct {
	commitments       []*privacy.EllipticPoint
	commitmentIndices []uint64
	index             byte
}

// OneOutOfManyWitness is a protocol for Zero-knowledge Proof of Knowledge of one out of many commitments containing 0
// include Witness: CommitedValue, r []byte
type OneOutOfManyWitness struct {
	stmt *OneOutOfManyStatement

	rand        *big.Int
	indexIsZero uint64
}

// OneOutOfManyProof contains Proof's value
type OneOutOfManyProof struct {
	stmt *OneOutOfManyStatement

	cl, ca, cb, cd []*privacy.EllipticPoint
	f, za, zb      []*big.Int
	zd             *big.Int
}

func (proof *OneOutOfManyProof) isNil() bool {
	if proof.cl == nil {
		return true
	}
	if proof.ca == nil {
		return true
	}
	if proof.cb == nil {
		return true
	}
	if proof.cd == nil {
		return true
	}
	if proof.f == nil {
		return true
	}
	if proof.za == nil {
		return true
	}
	if proof.zb == nil {
		return true
	}
	if proof.zd == nil {
		return true
	}
	return proof.stmt == nil
}

func (proof *OneOutOfManyProof) Init() *OneOutOfManyProof {
	proof.zd = new(big.Int)
	proof.stmt = new(OneOutOfManyStatement)

	return proof
}

// Set sets Statement
func (stmt *OneOutOfManyStatement) Set(
	commitments       []*privacy.EllipticPoint,
	commitmentIndices []uint64,
	index             byte) {
	stmt.commitments = commitments
	stmt.commitmentIndices = commitmentIndices
	stmt.index = index
}

// Set sets Witness
func (wit *OneOutOfManyWitness) Set(
	commitments []*privacy.EllipticPoint,
	commitmentIndices []uint64,
	rand *big.Int,
	indexIsZero uint64,
	index byte) {
	wit.stmt = new(OneOutOfManyStatement)
	wit.stmt.Set(commitments, commitmentIndices, index)

	wit.indexIsZero = indexIsZero
	wit.rand = rand
}

// Set sets Proof
func (proof *OneOutOfManyProof) Set(
	commitmentIndexs []uint64,
	commitments []*privacy.EllipticPoint,
	cl, ca, cb, cd []*privacy.EllipticPoint,
	f, za, zb []*big.Int,
	zd *big.Int,
	index byte) {

	proof.stmt = new(OneOutOfManyStatement)
	proof.stmt.Set(commitments, commitmentIndexs, index)

	proof.cl, proof.ca, proof.cb, proof.cd = cl, ca, cb, cd
	proof.f, proof.za, proof.zb = f, za, zb
	proof.zd = zd
}

// Bytes converts one of many proof to bytes array
func (proof *OneOutOfManyProof) Bytes() []byte {
	// if proof is nil, return an empty array
	if proof.isNil() {
		return []byte{}
	}

	// N = 2^n
	N := privacy.CMRingSize
	n := privacy.CMRingSizeExp

	var bytes []byte

	// convert array cl to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, proof.cl[i].Compress()...)
	}
	// convert array ca to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, proof.ca[i].Compress()...)
	}

	// convert array cb to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, proof.cb[i].Compress()...)
	}

	// convert array cd to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, proof.cd[i].Compress()...)
	}

	// convert array f to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, privacy.AddPaddingBigInt(proof.f[i], privacy.BigIntSize)...)
	}

	// convert array za to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, privacy.AddPaddingBigInt(proof.za[i], privacy.BigIntSize)...)
	}

	// convert array zb to bytes array
	for i := 0; i < n; i++ {
		bytes = append(bytes, privacy.AddPaddingBigInt(proof.zb[i], privacy.BigIntSize)...)
	}

	// convert array zd to bytes array
	bytes = append(bytes, privacy.AddPaddingBigInt(proof.zd, privacy.BigIntSize)...)

	// convert commitment index to bytes array
	for i := 0; i < N; i++ {
		commitmentIndexBytes := make([]byte, privacy.Uint64Size)
		binary.LittleEndian.PutUint64(commitmentIndexBytes, proof.stmt.commitmentIndices[i])
		bytes = append(bytes, commitmentIndexBytes...)
	}

	// append index
	bytes = append(bytes, proof.stmt.index)

	return bytes
}

// SetBytes convert from bytes array to OneOutOfManyProof
func (proof *OneOutOfManyProof) SetBytes(bytes []byte) error {
	if len(bytes) == 0 {
		return nil
	}

	N := privacy.CMRingSize
	n := privacy.CMRingSizeExp

	offset := 0
	var err error

	// get cl array
	proof.cl = make([]*privacy.EllipticPoint, n)
	for i := 0; i < n; i++ {
		proof.cl[i], err = privacy.DecompressKey(bytes[offset : offset+privacy.CompressedPointSize])
		if err != nil {
			return err
		}
		offset = offset + privacy.CompressedPointSize
	}

	// get ca array
	proof.ca = make([]*privacy.EllipticPoint, n)
	for i := 0; i < n; i++ {
		proof.ca[i], err = privacy.DecompressKey(bytes[offset : offset+privacy.CompressedPointSize])
		if err != nil {
			return err
		}
		offset = offset + privacy.CompressedPointSize
	}

	// get cb array
	proof.cb = make([]*privacy.EllipticPoint, n)
	for i := 0; i < n; i++ {
		proof.cb[i], err = privacy.DecompressKey(bytes[offset : offset+privacy.CompressedPointSize])
		if err != nil {
			return err
		}
		offset = offset + privacy.CompressedPointSize
	}

	// get cd array
	proof.cd = make([]*privacy.EllipticPoint, n)
	for i := 0; i < n; i++ {
		proof.cd[i], err = privacy.DecompressKey(bytes[offset : offset+privacy.CompressedPointSize])
		if err != nil {
			return err
		}
		offset = offset + privacy.CompressedPointSize
	}

	// get f array
	proof.f = make([]*big.Int, n)
	for i := 0; i < n; i++ {
		proof.f[i] = new(big.Int).SetBytes(bytes[offset : offset+privacy.BigIntSize])
		offset = offset + privacy.BigIntSize
	}

	// get za array
	proof.za = make([]*big.Int, n)
	for i := 0; i < n; i++ {
		proof.za[i] = new(big.Int).SetBytes(bytes[offset : offset+privacy.BigIntSize])
		offset = offset + privacy.BigIntSize
	}

	// get zb array
	proof.zb = make([]*big.Int, n)
	for i := 0; i < n; i++ {
		proof.zb[i] = new(big.Int).SetBytes(bytes[offset : offset+privacy.BigIntSize])
		offset = offset + privacy.BigIntSize
	}

	// get zd
	proof.zd = new(big.Int).SetBytes(bytes[offset : offset+privacy.BigIntSize])
	offset = offset + privacy.BigIntSize
	// get commitments list
	proof.stmt.commitmentIndices = make([]uint64, N)
	for i := 0; i < N; i++ {
		proof.stmt.commitmentIndices[i] = binary.LittleEndian.Uint64(bytes[offset : offset+privacy.Uint64Size])
		offset = offset + privacy.Uint64Size
	}

	//get index
	proof.stmt.index = bytes[len(bytes)-1]
	return nil
}

// Prove creates proof for one out of many commitments containing 0
func (wit *OneOutOfManyWitness) Prove() (*OneOutOfManyProof, error) {
	start :=time.Now()
	// Check the number of Commitment list's elements
	N := len(wit.stmt.commitments)
	if N != privacy.CMRingSize {
		return nil, errors.New("the number of Commitment list's elements must be equal to CMRingSize")
	}

	n := privacy.CMRingSizeExp

	// Check indexIsZero
	if wit.indexIsZero > uint64(N) {
		return nil, errors.New("Index is zero must be Index in list of commitments")
	}

	// Check Index
	if wit.stmt.index < privacy.SK || wit.stmt.index > privacy.RAND {
		return nil, errors.New("Index must be between index sk and index RAND")
	}

	// represent indexIsZero in binary
	indexIsZeroBinary := privacy.ConvertIntToBinary(int(wit.indexIsZero), n)

	//
	r := make([]*big.Int, n)
	a := make([]*big.Int, n)
	s := make([]*big.Int, n)
	t := make([]*big.Int, n)
	u := make([]*big.Int, n)

	cl := make([]*privacy.EllipticPoint, n)
	ca := make([]*privacy.EllipticPoint, n)
	cb := make([]*privacy.EllipticPoint, n)
	cd := make([]*privacy.EllipticPoint, n)

	for j := 0; j <n; j++ {
		// Generate random numbers
		r[j] = privacy.RandInt()
		a[j] = privacy.RandInt()
		s[j] = privacy.RandInt()
		t[j] = privacy.RandInt()
		u[j] = privacy.RandInt()

		// convert indexIsZeroBinary[j] to big.Int
		indexInt := big.NewInt(int64(indexIsZeroBinary[j]))

		// Calculate cl, ca, cb, cd
		// cl = Com(l, r)
		cl[j] = privacy.PedCom.CommitAtIndex(indexInt, r[j], wit.stmt.index)

		// ca = Com(a, s)
		ca[j] = privacy.PedCom.CommitAtIndex(a[j], s[j], wit.stmt.index)

		// cb = Com(la, t)
		la := new(big.Int).Mul(indexInt, a[j])
		la.Mod(la, privacy.Curve.Params().N)
		cb[j] = privacy.PedCom.CommitAtIndex(la, t[j], wit.stmt.index)
	}

	// Calculate: cd_k = ci^pi,k
	for k := 0; k < n; k++ {
		// Calculate pi,k which is coefficient of x^k in polynomial pi(x)
		cd[k] = new(privacy.EllipticPoint).Zero()

		for i := 0; i < N; i++ {
			iBinary := privacy.ConvertIntToBinary(i, n)
			pik := GetCoefficient(iBinary, k, n, a, indexIsZeroBinary)
			cd[k] = cd[k].Add(wit.stmt.commitments[i].ScalarMult(pik))
		}

		cd[k] = cd[k].Add(privacy.PedCom.CommitAtIndex(big.NewInt(0), u[k], wit.stmt.index))
	}

	// Calculate x
	x := big.NewInt(0)
	for j := 0; j <= n-1; j++ {
		x = generateChallengeFromByte([][]byte{x.Bytes(), cl[j].Compress(), ca[j].Compress(), cb[j].Compress(), cd[j].Compress()})
	}

	// Calculate za, zb zd
	za := make([]*big.Int, n)
	zb := make([]*big.Int, n)
	zd := new(big.Int)
	f := make([]*big.Int, n)

	for j := 0; j < n; j++ {
		// f = lx + a
		f[j] = new(big.Int).Mul(big.NewInt(int64(indexIsZeroBinary[j])), x)
		f[j].Add(f[j], a[j])
		f[j].Mod(f[j], privacy.Curve.Params().N)

		// za = s + rx
		za[j] = new(big.Int).Mul(r[j], x)
		za[j].Add(za[j], s[j])
		za[j].Mod(za[j], privacy.Curve.Params().N)

		// zb = r(x - f) + t
		zb[j] = new(big.Int).Sub(x, f[j])
		zb[j].Mul(zb[j], r[j])
		zb[j].Add(zb[j], t[j])
		zb[j].Mod(zb[j], privacy.Curve.Params().N)
	}

	// zd = rand * x^n - sum_{k=0}^{n-1} u[k] * x^k
	zd.Exp(x, big.NewInt(int64(n)), privacy.Curve.Params().N)
	zd.Mul(zd, wit.rand)

	uxInt := big.NewInt(0)
	sumInt := big.NewInt(0)
	for k := 0; k < n; k++ {
		uxInt.Exp(x, big.NewInt(int64(k)), privacy.Curve.Params().N)
		uxInt.Mul(uxInt, u[k])
		sumInt.Add(sumInt, uxInt)
		sumInt.Mod(sumInt, privacy.Curve.Params().N)
	}

	zd.Sub(zd, sumInt)
	zd.Mod(zd, privacy.Curve.Params().N)

	proof := new(OneOutOfManyProof).Init()
	proof.Set(wit.stmt.commitmentIndices, wit.stmt.commitments, cl, ca, cb, cd, f, za, zb, zd, wit.stmt.index)

	end := time.Since(start)
	fmt.Printf("One out of many proving time: %v\n", end)
	return proof, nil
}

func (proof *OneOutOfManyProof) Verify() bool {
	N := len(proof.stmt.commitments)

	// the number of Commitment list's elements must be equal to CMRingSize
	if N != privacy.CMRingSize {
		return false
	}
	n := privacy.CMRingSizeExp

	//Calculate x
	x := big.NewInt(0)

	for j := 0 ; j <= n-1; j++ {
		x = generateChallengeFromByte([][]byte{x.Bytes(), proof.cl[j].Compress(), proof.ca[j].Compress(), proof.cb[j].Compress(), proof.cd[j].Compress()})
	}

	for i := 0; i < n; i++ {
		// Check cl^x * ca = Com(f, za)
		leftPoint1 := proof.cl[i].ScalarMult(x).Add(proof.ca[i])
		rightPoint1 := privacy.PedCom.CommitAtIndex(proof.f[i], proof.za[i], proof.stmt.index)

		if !leftPoint1.IsEqual(rightPoint1) {
			return false
		}

		// Check cl^(x-f) * cb = Com(0, zb)
		xSubF := new(big.Int).Sub(x, proof.f[i])
		xSubF.Mod(xSubF, privacy.Curve.Params().N)

		leftPoint2 := proof.cl[i].ScalarMult(xSubF).Add(proof.cb[i])
		rightPoint2 := privacy.PedCom.CommitAtIndex(big.NewInt(0), proof.zb[i], proof.stmt.index)

		if !leftPoint2.IsEqual(rightPoint2) {
			return false
		}
	}

	leftPoint3 := new(privacy.EllipticPoint).Zero()
	leftPoint32 := new(privacy.EllipticPoint).Zero()

	for i := 0; i < N; i++ {
		iBinary := privacy.ConvertIntToBinary(i, n)

		exp := big.NewInt(1)
		fji := big.NewInt(1)
		for j := 0; j < n; j++ {
			if iBinary[j] == 1 {
				fji.Set(proof.f[j])
			} else {
				fji.Sub(x, proof.f[j])
				fji.Mod(fji, privacy.Curve.Params().N)
			}

			exp.Mul(exp, fji)
			exp.Mod(exp, privacy.Curve.Params().N)
		}

		leftPoint3 = leftPoint3.Add(proof.stmt.commitments[i].ScalarMult(exp))
	}

	for k := 0; k < n; k++ {
		xk := big.NewInt(0).Exp(x, big.NewInt(int64(k)), privacy.Curve.Params().N)
		xk.Sub(privacy.Curve.Params().N, xk)

		leftPoint32 = leftPoint32.Add(proof.cd[k].ScalarMult(xk))
	}

	leftPoint3 = leftPoint3.Add(leftPoint32)

	rightPoint3 := privacy.PedCom.CommitAtIndex(big.NewInt(0), proof.zd, proof.stmt.index)

	return leftPoint3.IsEqual(rightPoint3)
}

// Get coefficient of x^k in polynomial pi(x)
func GetCoefficient(iBinary []byte, k int, n int, a []*big.Int, l []byte) *big.Int {
	res := privacy.Poly{big.NewInt(1)}
	var fji privacy.Poly

	for j := n - 1; j >= 0; j-- {
		fj := privacy.Poly{a[j], big.NewInt(int64(l[j]))}
		if iBinary[j] == 0 {
			fji = privacy.Poly{big.NewInt(0), big.NewInt(1)}.Sub(fj, privacy.Curve.Params().N)
		} else {
			fji = fj
		}
		res = res.Mul(fji, privacy.Curve.Params().N)
	}

	if res.GetDegree() < k {
		return big.NewInt(0)
	}
	return res[k]
}
