package transaction

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/go-sdk/pkg/address"
	"github.com/harmony-one/go-sdk/pkg/common"
	"github.com/harmony-one/go-sdk/pkg/ledger"
	"github.com/harmony-one/go-sdk/pkg/rpc"
	"github.com/harmony-one/harmony/accounts"
	"github.com/harmony-one/harmony/accounts/keystore"
	"github.com/harmony-one/harmony/common/denominations"
	"github.com/harmony-one/harmony/core"
	staking "github.com/harmony-one/harmony/staking/types"
)

type p []interface{}

type transactionForRPC struct {
	params      map[string]interface{}
	shardTx 		*Transaction
	stakeTx			*staking.StakingTransaction
	// Hex encoded
	signature   *string
	receiptHash *string
	receipt     rpc.Reply
}

type sender struct {
	ks      *keystore.KeyStore
	account *accounts.Account
}

// Controller drives the transaction signing process
type Controller struct {
	failure           error
	messenger         rpc.T
	sender            sender
	transactionForRPC transactionForRPC
	chain             common.ChainID
	Behavior          behavior
}

type behavior struct {
	DryRun               bool
	SigningImpl          SignerImpl
	ConfirmationWaitTime uint32
}

// NewController initializes a Controller, caller can control behavior via options
func NewController(
	handler rpc.T,
	senderKs *keystore.KeyStore,
	senderAcct *accounts.Account,
	chain common.ChainID,
	transactionType string,
	options ...func(*Controller)) *Controller {

	txParams := make(map[string]interface{})
	ctrlr := &Controller{
		failure:   nil,
		messenger: handler,
		sender: sender{
			ks:      senderKs,
			account: senderAcct,
		},
		transactionForRPC: transactionForRPC{
			params:      txParams,
			signature:   nil,
			receiptHash: nil,
			receipt:     nil,
		},
		chain:    chain,
		Behavior: behavior{false, Software, 0},
	}
	for _, option := range options {
		option(ctrlr)
	}
	return ctrlr
}

func (C *Controller) verifyBalance(amount float64) {
	if C.failure != nil {
		return
	}
	balanceRPCReply, err := C.messenger.SendRPC(
		rpc.Method.GetBalance,
		p{address.ToBech32(C.sender.account.Address), "latest"},
	)
	if err != nil {
		C.failure = err
		return
	}
	currentBalance, _ := balanceRPCReply["result"].(string)
	balance, _ := big.NewInt(0).SetString(currentBalance[2:], 16)
	balance = common.NormalizeAmount(balance)
	transfer := big.NewInt(int64(amount * denominations.Nano))

	tns := (float64(transfer.Uint64()) / denominations.Nano)
	bln := (float64(balance.Uint64()) / denominations.Nano)

	if tns > bln {
		C.failure = fmt.Errorf(
			"current balance of %.6f is not enough for the requested transfer %.6f", bln, tns,
		)
	}
}

func (C *Controller) setNextNonce() {
	if C.failure != nil {
		return
	}
	transactionCountRPCReply, err :=
		C.messenger.SendRPC(rpc.Method.GetTransactionCount, p{C.sender.account.Address.Hex(), "latest"})
	if err != nil {
		C.failure = err
		return
	}
	transactionCount, _ := transactionCountRPCReply["result"].(string)
	nonce, _ := big.NewInt(0).SetString(transactionCount[2:], 16)
	C.transactionForRPC.params["nonce"] = nonce.Uint64()
}

func (C *Controller) sendSignedTx() {
	if C.failure != nil || C.Behavior.DryRun {
		return
	}
	reply, err := C.messenger.SendRPC(rpc.Method.SendRawTransaction, p{C.transactionForRPC.signature})
	if err != nil {
		C.failure = err
		return
	}
	r, _ := reply["result"].(string)
	C.transactionForRPC.receiptHash = &r
}

func (C *Controller) setIntrinsicGas(rawInput string) {
	if C.failure != nil {
		return
	}
	inputData, _ := base64.StdEncoding.DecodeString(rawInput)
	gas, _ := core.IntrinsicGas(inputData, false, true)
	C.transactionForRPC.params["gas"] = gas
}

func (C *Controller) setGasPrice() {
	if C.failure != nil {
		return
	}
	C.transactionForRPC.params["gas-price"] = nil
}

func (C *Controller) setAmount(amount float64) {
	amountBigInt := big.NewInt(int64(amount * denominations.Nano))
	amt := amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	C.transactionForRPC.params["transfer-amount"] = amt
}

func (C *Controller) setReceiver(receiver string) {
	C.transactionForRPC.params["receiver"] = address.Parse(receiver)
}

func (C *Controller) setNewTransactionWithDataAndGas(i string, amount, gasPrice float64) {
	if C.failure != nil {
		return
	}
	amountBigInt := big.NewInt(int64(amount * denominations.Nano))
	amt := amountBigInt.Mul(amountBigInt, big.NewInt(denominations.Nano))
	gPrice := big.NewInt(int64(gasPrice))
	gPrice = gPrice.Mul(gPrice, big.NewInt(denominations.Nano))

	tx := NewTransaction(
		C.transactionForRPC.params["nonce"].(uint64),
		C.transactionForRPC.params["gas"].(uint64),
		C.transactionForRPC.params["receiver"].(address.T),
		C.transactionForRPC.params["from-shard"].(uint32),
		C.transactionForRPC.params["to-shard"].(uint32),
		amt,
		gPrice,
		[]byte(i),
	)
	C.transactionForRPC.shardTx = tx
}

func (C *Controller) setNewStakingTransaction(gasPrice float64) {
	if C.failure != nil {
		return
	}
	gPrice := big.NewInt(int64(gasPrice))
	gPrice = gPrice.Mul(gPrice, big.NewInt(denominations.Nano))

	stakingTx, err := staking.NewStakingTransaction(
		C.transactionForRPC.params["nonce"].(uint64),
		100, // gasLimit?
		gPrice,
		C.transactionForRPC.params["directive"].(Fulfill),
	)
	if err != nil {
		C.failure = err
		return
	}
	C.transactionForRPC.stakeTx = stakingTx
}

func (C *Controller) TransactionToJSON(pretty bool) string {
	r, _ := C.transactionForRPC.shardTx.MarshalJSON()
	if pretty {
		return common.JSONPrettyFormat(string(r))
	}
	return string(r)
}

func (C *Controller) signAndPrepareTxEncodedForSending() {
	if C.failure != nil {
		return
	}
	signedTransaction, err :=
		C.sender.ks.SignTx(*C.sender.account, C.transactionForRPC.shardTx, C.chain.Value)
	if err != nil {
		C.failure = err
		return
	}
	C.transactionForRPC.shardTx = signedTransaction
	enc, _ := rlp.EncodeToBytes(signedTransaction)
	hexSignature := hexutil.Encode(enc)
	C.transactionForRPC.signature = &hexSignature
	if common.DebugTransaction {
		r, _ := signedTransaction.MarshalJSON()
		fmt.Println("Signed with ChainID:", C.transactionForRPC.shardTx.ChainID())
		fmt.Println(common.JSONPrettyFormat(string(r)))
	}
}

func (C *Controller) signAndPrepareStakingTxEncodedForSending() {
	if C.failure != nil {
		return
	}
	signedTransaction, err :=
			C.sender.ks.SignStakingTx(*C.sender.account, C.transactionForRPC.stakeTx, C.chain.Value)
	if err != nil {
		C.failure = err
		return
	}
	C.transactionForRPC.stakeTx = signedTransaction
	enc, _ := rlp.EncodeToBytes(signedTransaction)
	hexSignature := hexutil.Encode(enc)
	C.transactionForRPC.signature = &hexSignature
	// if common.DebugTransaction {
	// 	r, _ := signedTransaction.MarshalJSON()
	// 	fmt.Println("Signed with ChainID:", C.transactionForRPC.stakeTx.ChainID())
	// 	fmt.Println(common.JSONPrettyFormat(string(r)))
	// }
}

func (C *Controller) setShardIDs(fromShard, toShard int) {
	if C.failure != nil {
		return
	}
	C.transactionForRPC.params["from-shard"] = uint32(fromShard)
	C.transactionForRPC.params["to-shard"] = uint32(toShard)
}

func (C *Controller) ReceiptHash() *string {
	return C.transactionForRPC.receiptHash
}

func (C *Controller) Receipt() rpc.Reply {
	return C.transactionForRPC.receipt
}

func (C *Controller) hardwareSignAndPrepareTxEncodedForSending() {
	if C.failure != nil {
		return
	}
	enc, signerAddr, err := ledger.SignTx(C.transactionForRPC.shardTx, C.chain.Value)
	if err != nil {
		C.failure = err
		return
	}
	if strings.Compare(signerAddr, address.ToBech32(C.sender.account.Address)) != 0 {
		C.failure = errors.New("signature verification failed : sender address doesn't match with ledger hardware addresss")
		return
	}
	hexSignature := hexutil.Encode(enc)
	C.transactionForRPC.signature = &hexSignature
}

func (C *Controller) txConfirmation() {
	if C.failure != nil {
		return
	}
	if C.Behavior.ConfirmationWaitTime > 0 {
		receipt := *C.ReceiptHash()
		start := int(C.Behavior.ConfirmationWaitTime)
		for {
			if start < 0 {
				return
			}
			r, _ := C.messenger.SendRPC(rpc.Method.GetTransactionReceipt, p{receipt})
			if r["result"] != nil {
				C.transactionForRPC.receipt = r
				return
			}
			time.Sleep(time.Second * 2)
			start = start - 2
		}
	}
}

	// func (C *Controller) setValidatorDescription(
	// 	name, identity, website, secContract, details string,
	// 	) {
	// 	desc := staking.Description {
	// 		name,
	// 		identity,
	// 		website,
	// 		secContact,
	// 		details,
	// 	}
	// 	C.transactionForRPC.params["description"] = desc
	// }
	//
	// func (C *Controller) setCommissionRates(
	// 	rate, maxRate, maxChangeRate float64,
	// 	) {
	// 	cr := staking.CommissionRates {
	// 		staking.NewDec(rate),
	// 		staking.NewDec(maxRate),
	// 		staking.NewDec(maxChangeRate),
	// 	}
	// 	C.transactionForRPC.params["commissionrates"] = cr
	// }
	//
	// func (C *Controller) setNewValidatorDirective(
	// 	pubkey string,
	// 	minSelfDelegation, amount float64,
	// 	) {
	// 	if C.failure != nil {
	// 		return
	// 	}
	// 	p := &bls.PublicKey{}
	// 	p.DeserializeHexStr(pubKey)
	// 	pub := shard.BlsPublicKey{}
	// 	pub.FromLibBLSPublicKey(p)
	// 	stakePayloadMaker := func() (staking.Directive, interface{}) {
	// 		return staking.DirectiveNewValidator, staking.NewValidator{
	// 			C.transactionForRPC.params["directive"],
	// 			C.transactionForRPC.params["commissionrates"],
	// 			big.NewInt(minSelfDelegation),
	// 			C.sender.account.Address,
	// 			pub,
	// 			big.NewInt(amount),
	// 		}
	// 	C.transactionForRPC.params["directive"] = stakePayloadMaker
	// }
	//
	// func (C *Controller) setEditValidatorDirective() {
	// 	if C.failure != nil {
	// 		return
	// 	}
	// 	stakePayloadMaker := func() (staking.Directive, interface{}) {
	// 		return staking.DirectivUndelegate, staking.Undelegate{
	// 			C.sender.account.Address,
	// 			address.Parse(addr),
	// 			big.NewInt(amount),
	// 		}
	// 	C.transactionForRPC.params["directive"] = stakePayloadMaker
	// }
	//
	func (C *Controller) setDelegateDirective(addr string, amount float64) {
		stakePayloadMaker := func() (staking.Directive, interface{}) {
			return staking.DirectiveDelegate, staking.Delegate{
				C.sender.account.Address,
				address.Parse(addr),
				big.NewInt(int64(amount)),
			}
		}
		C.transactionForRPC.params["directive"] = stakePayloadMaker
	}
	//
	// func (C *Controller) setRedelegateDirective(srcaddr, destaddr string, amount float64) {
	// 	if C.failure != nil {
	// 		return
	// 	}
	// 	stakePayloadMaker := func() (staking.Directive, interface{}) {
	// 		return staking.DirectiveRedelegate, staking.Redelegate{
	// 			C.sender.account.Address,
	// 			address.Parse(srcaddr),
	// 			address.Parse(destaddr),
	// 			big.NewInt(amount),
	// 		}
	// 	}
	// 	C.transactionForRPC.params["directive"] = stakePayloadMaker
	// }
	//
	// func (C *Controller) setUndelegateDirective(addr string, amount float64) {
	// 	if C.failure != nil {
	// 		return
	// 	}
	// 	stakePayloadMaker := func() (staking.Directive, interface{}) {
	// 		return staking.DirectiveUndelegate, staking.Undelegate{
	// 			C.sender.account.Address,
	// 			address.Parse(addr),
	// 			big.NewInt(amount),
	// 		}
	// 	}
	// 	C.transactionForRPC.params["directive"] = stakePayloadMaker
	// }

func (C *Controller) ExecuteShardingTransaction(
	to, inputData string,
	amount, gPrice float64,
	fromShard, toShard int,
) error {
	// WARNING Order of execution matters
		C.setShardIDs(fromShard, toShard)
		C.setIntrinsicGas(inputData)
		C.setAmount(amount)
		C.verifyBalance(amount)
		C.setReceiver(to)
		C.setGasPrice()
		C.setNextNonce()
		C.setNewTransactionWithDataAndGas(inputData, amount, gPrice)
	switch C.Behavior.SigningImpl {
	case Software:
		C.signAndPrepareTxEncodedForSending()
	case Ledger:
		C.hardwareSignAndPrepareTxEncodedForSending()
	}
	C.sendSignedTx()
	C.txConfirmation()
	return C.failure
}

// func (C *Controller) ExecuteNewValidatorTransaction (
// 	name, identity, website, secContract, dets, pubKey string,
// 	rate, maxRate, maxChangeRate, minSelfDelegation, amount, gasPrice float64,
// 	) error {
// 	C.setValidatorDescription(name, identity, website, secContract, dets)
// 	C.setCommissionRates(rate, maxRate, maxChangeRate)
// 	C.setNewValidatorDirective(pubKey, minSelfDelegation, amount)
// 	C.setAmount(amount)
// 	C.verifyBalance(amount)
// 	C.setGasPrice()
// 	C.setNextNonce()
// 	C.setNewStakingTransaction()
// 	C.signAndPrepareTxEncodedForSending()
// 	C.sendSignedTx()
// 	C.txConfirmation()
// 	return C.failure
// }

func (C *Controller) ExecuteDelegateTransaction (
	addr string,
	amount, gasPrice float64,
	) error {
	C.setDelegateDirective(addr, amount)
	C.setAmount(amount)
	C.verifyBalance(amount)
	C.setGasPrice()
	C.setNextNonce()
	C.setNewStakingTransaction(gasPrice)
	C.signAndPrepareStakingTxEncodedForSending()
	C.sendSignedTx()
	C.txConfirmation()
	return C.failure
}
