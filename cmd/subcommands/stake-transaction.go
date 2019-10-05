package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var (
	StakingTransactionCommand *cobra.Command
)

func init() {
	rootStakingTxnCmd := &cobra.Command{
		Use:   "stake",
		Short: "Send a staking transaction",
		Long:  `Create a transaction, sign it, and send off to the Harmony blockchain`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			fmt.Println("TODO: logic for staking transaction")
		},
	}

	StakingTransactionCommand = rootStakingTxnCmd
}
