package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "go-easy-instrumentation",
	Short: "go-easy-instrumentation adds basic application monitoring instrumentation API calls to your program source code",
	Long:  "go-easy-instrumentation adds basic application monitoring instrumentation API calls to your program source code",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(`go-easy-instrumentation helps you add instrumentation code using the New Relic
Go APM agent API. To add instrumentation for the first time to an existing 
application, invoke the "instrument" subcommand as:
  $ go-easy-instrumentation instrument <path> [flags]
where <path> is the path to the application code.

For more information, use the "help" subcommand, the "--help" flag with any
subcommand, or the online documentation from New Relic.`)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
