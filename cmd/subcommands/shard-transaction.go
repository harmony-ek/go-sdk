package cmd

import (
	"fmt"
	"github.com/harmony-one/go-sdk/pkg/address"
	"github.com/harmony-one/go-sdk/pkg/common"
	"github.com/harmony-one/go-sdk/pkg/rpc"
	"github.com/harmony-one/go-sdk/pkg/sharding"
	"github.com/harmony-one/go-sdk/pkg/store"
	"github.com/harmony-one/go-sdk/pkg/transaction"
	"github.com/harmony-one/harmony/accounts"
	"github.com/spf13/cobra"
)

var (
	shardTransactionCommand *cobra.Command

	dryRun      bool
	fromAddress oneAddress
	toAddress   oneAddress
	amount      float64
	fromShardID int
	toShardID   int
	confirmWait uint32
	chainName   = chainIDWrapper{chainID: &common.Chain.TestNet}
	gasPrice    float64
)

func handlerForShard(senderShard int, node string) (*rpc.HTTPMessenger, error) {
	s, err := sharding.Structure(node)
	if err != nil {
		return nil, err
	}
	for _, shard := range s {
		if shard.ShardID == senderShard {
			return rpc.NewHTTPHandler(shard.HTTP), nil
		}
	}
	return nil, nil
}

func opts(ctlr *transaction.Controller) {
	if dryRun {
		ctlr.Behavior.DryRun = true
	}
	if useLedgerWallet {
		ctlr.Behavior.SigningImpl = transaction.Ledger
	}
	if confirmWait > 0 {
		ctlr.Behavior.ConfirmationWaitTime = confirmWait
	}
}

func init() {
	rootShardTxnCmd := &cobra.Command{
		Use:   "shard",
		Short: "Send a transaction across or within a shard",
		Long:  `Create a transaction, sign it, and send off to the Harmony blockchain`,
		RunE: func(cmd *cobra.Command, args []string) error {
			from := fromAddress.String()
			networkHandler, err := handlerForShard(fromShardID, node)
			if err != nil {
				return err
			}
			var ctrlr *transaction.Controller
			if useLedgerWallet {
				account := accounts.Account{Address: address.Parse(from)}
				ctrlr = transaction.NewController(networkHandler, nil, &account, *chainName.chainID, opts)
			} else {
				ks, acct, err := store.UnlockedKeystore(from, UnlockPassphrase)
				if err != nil {
					return err
				}
				ctrlr = transaction.NewController(networkHandler, ks, acct, *chainName.chainID, opts)
			}

			if transactionFailure := ctrlr.ExecuteShardingTransaction(
				toAddress.String(),
				"",
				amount, gasPrice,
				fromShardID,
				toShardID,
			); transactionFailure != nil {
				return transactionFailure
			}
			switch {
			case !dryRun && confirmWait == 0:
				fmt.Println(fmt.Sprintf(`{"transaction-receipt":"%s"}`, *ctrlr.ReceiptHash()))
			case !dryRun && confirmWait > 0:
				fmt.Println(common.ToJSONUnsafe(ctrlr.Receipt(), !noPrettyOutput))
			case dryRun:
				fmt.Println(ctrlr.TransactionToJSON(!noPrettyOutput))
			}
			return nil
		},
	}

	rootShardTxnCmd.Flags().Var(&fromAddress, "from", "sender's one address, keystore must exist locally")
	rootShardTxnCmd.Flags().Var(&toAddress, "to", "the destination one address")
	rootShardTxnCmd.Flags().BoolVar(&dryRun, "dry-run", false, "do not send signed transaction")
	rootShardTxnCmd.Flags().Float64Var(&amount, "amount", 0.0, "amount")
	rootShardTxnCmd.Flags().Float64Var(&gasPrice, "gas-price", 0.0, "gas price to pay")
	rootShardTxnCmd.Flags().IntVar(&fromShardID, "from-shard", -1, "source shard id")
	rootShardTxnCmd.Flags().IntVar(&toShardID, "to-shard", -1, "target shard id")
	rootShardTxnCmd.Flags().Var(&chainName, "chain-id", "what chain ID to target")
	rootShardTxnCmd.Flags().Uint32Var(&confirmWait, "wait-for-confirm", 0, "only waits if non-zero value, in seconds")
	for _, flagName := range [...]string{"from", "to", "amount", "from-shard", "to-shard"} {
		rootShardTxnCmd.MarkFlagRequired(flagName)
	}
	shardTransactionCommand = rootShardTxnCmd
}
