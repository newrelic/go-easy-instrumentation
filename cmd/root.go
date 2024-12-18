package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "go-easy-instrumentation",
	Short: "go-easy-instrumentation adds basic application monitoring instrumentation API calls to your program source code",
	Long:  "go-easy-instrumentation adds basic application monitoring instrumentation API calls to your program source code",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}
