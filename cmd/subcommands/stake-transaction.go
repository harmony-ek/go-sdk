package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/harmony-one/go-sdk/pkg/address"
	"github.com/harmony-one/go-sdk/pkg/common"
	"github.com/harmony-one/go-sdk/pkg/rpc"
	"github.com/harmony-one/go-sdk/pkg/sharding"
	"github.com/harmony-one/go-sdk/pkg/store"
	"github.com/harmony-one/go-sdk/pkg/transaction"
	"github.com/harmony-one/harmony/accounts"
	staking "github.com/harmony-one/harmony/staking/types"
)

var (
	stakingTransactionCommand *cobra.Command

	nonce				uint64
	dryRun      bool
	fromAddress oneAddress
	toAddress   oneAddress
	amount      float64
	confirmWait uint32
)

func init() {
	rootStakingTxnCmd := &cobra.Command{
		Use:   "stake",
		Short: "Send a staking transaction",
		Long:  `Create a transaction, sign it, and send off to the Harmony blockchain`,
		Run: func(cmd *cobra.Command, args []string) {
			daddr := delAddress.String()
			networkHandler, err := handlerForShard(0, node) // Staking transactions only on Shard 0
			if err != nil {
				return err
			}
			var ctrlr *transaction.Controller
			if useLedgerWallet {
				account := accounts.Account{Address: address.Parse(daddr)}
				ctrlr = transaction.NewController(networkHandler, nil, &account, *chainName.chainID, opts)
			} else {
				ks, acct, err := store.UnlockedKeystore(from, UnlockPassphrase)
				if err != nil {
					return err
				}
				ctrlr = transaction.NewController(networkHandler, ks, acct, *chainName.chainID, opts)
			}

			if transactionFailure := ctrlr.ExecuteDelegateTransaction(
				valAddress.String(),
				amount, gasPrice,
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

	rootStakingTxnCmd.Flags().Var(&msgType, "tx-type", "directive of staking transaction")
	rootStakingTxnCmd.Flags().Var(&delAddress, "del", "delegator's one address, keystore must exist locally")
	rootStakingTxnCmd.Flags().Var(&valAddress, "val", "the validator one address")
	rootStakingTxnCmd.Flags().BoolVar(&dryRun, "dry-run", false, "do not send signed transaction")
	rootStakingTxnCmd.Flags().Float64Var(&amount, "amount", 0.0, "amount")
	rootStakingTxnCmd.Flags().Float64Var(&gasPrice, "gas-price", 0.0, "gas price to pay")
	rootStakingTxnCmd.Flags().Var(&chainName, "chain-id", "what chain ID to target")
	rootStakingTxnCmd.Flags().Uint32Var(&confirmWait, "wait-for-confirm", 0, "only waits if non-zero value, in seconds")
	for _, flagName := range [...]string{"from", "to", "amount", "from-shard", "to-shard"} {
		rootStakingTxnCmd.MarkFlagRequired(flagName)
	}
	stakingTransactionCommand = rootStakingTxnCmd
}
