package types

import (
	"fmt"
	"github.com/spacemeshos/ed25519"
	"github.com/spacemeshos/go-spacemesh/log"
	"github.com/spacemeshos/go-spacemesh/signing"
)

type TransactionId Hash32

func (id TransactionId) Hash32() Hash32 {
	return Hash32(id)
}

func (id TransactionId) ShortString() string {
	return id.Hash32().ShortString()
}

func (id TransactionId) String() string {
	return id.Hash32().String()
}

func (id TransactionId) Bytes() []byte {
	return id[:]
}

var EmptyTransactionId = TransactionId{}

type Transaction struct {
	InnerTransaction
	Signature [64]byte
	origin    *Address
	id        *TransactionId
}

func (t *Transaction) Origin() Address {
	if t.origin == nil {
		panic("origin not set")
	}
	return *t.origin
}

func (t *Transaction) SetOrigin(origin Address) {
	t.origin = &origin
}

func (t *Transaction) CalcAndSetOrigin() error {
	txBytes, err := InterfaceToBytes(&t.InnerTransaction)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %v", err)
	}
	pubKey, err := ed25519.ExtractPublicKey(txBytes, t.Signature[:])
	if err != nil {
		return fmt.Errorf("failed to extract transaction pubkey: %v", err)
	}

	t.origin = &Address{}
	t.origin.SetBytes(pubKey)
	return nil
}

func (t *Transaction) Id() TransactionId {
	if t.id != nil {
		return *t.id
	}

	txBytes, err := InterfaceToBytes(t)
	if err != nil {
		panic("failed to marshal transaction: " + err.Error())
	}
	id := TransactionId(CalcHash32(txBytes))
	t.id = &id
	return id
}

func (t *Transaction) Hash32() Hash32 {
	return t.Id().Hash32()
}

func (t *Transaction) ShortString() string {
	return t.Id().ShortString()
}

func (t *Transaction) String() string {
	return fmt.Sprintf("<id: %s, origin: %s, recipient: %s, amount: %v, nonce: %v, gas_limit: %v, fee: %v>",
		t.Id().ShortString(), t.Origin().Short(), t.Recipient.Short(), t.Amount, t.AccountNonce, t.GasLimit, t.Fee)
}

type InnerTransaction struct {
	AccountNonce uint64
	Recipient    Address
	GasLimit     uint64
	Fee          uint64
	Amount       uint64
}

// TEST ONLY
func NewTxWithOrigin(nonce uint64, orig, rec Address, amount, gasLimit, fee uint64) *Transaction {
	inner := InnerTransaction{
		AccountNonce: nonce,
		Recipient:    rec,
		Amount:       amount,
		GasLimit:     gasLimit,
		Fee:          fee,
	}
	return &Transaction{
		InnerTransaction: inner,
		origin:           &orig,
	}
}

// TEST ONLY
func NewSignedTx(nonce uint64, rec Address, amount, gas, fee uint64, signer *signing.EdSigner) (*Transaction, error) {
	inner := InnerTransaction{
		AccountNonce: nonce,
		Recipient:    rec,
		Amount:       amount,
		GasLimit:     gas,
		Fee:          fee,
	}

	buf, err := InterfaceToBytes(&inner)
	if err != nil {
		log.Error("failed to marshal tx")
		return nil, err
	}

	sst := &Transaction{
		InnerTransaction: inner,
		Signature:        [64]byte{},
		origin:           &Address{},
	}

	copy(sst.Signature[:], signer.Sign(buf))
	sst.origin.SetBytes(signer.PublicKey().Bytes())

	return sst, nil
}

type Reward struct {
	Layer  LayerID
	Amount uint64
}
