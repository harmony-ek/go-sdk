package cmd

import (
	"github.com/harmony-one/go-sdk/pkg/common"
	"github.com/spf13/cobra"
)

var (
	UnlockPassphrase string
)

func init() {
	cmdTransaction := &cobra.Command{
		Use:   "transaction",
		Short: "Create and send a transaction",
		Long: `Create either a staking or sharded transaction dictated by argument 
`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	cmdTransaction.PersistentFlags().StringVar(&UnlockPassphrase,
		"passphrase", common.DefaultPassphrase,
		"passphrase to unlock sender's keystore",
	)

	cmdTransaction.AddCommand(ShardTransactionCommand)
	cmdTransaction.AddCommand(StakingTransactionCommand)
	RootCmd.AddCommand(cmdTransaction)
}
