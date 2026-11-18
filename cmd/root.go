package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "go-easy-instrumentation",
	Short:   "go-easy-instrumentation adds basic application monitoring instrumentation API calls to your program source code",
	Long:    "go-easy-instrumentation adds basic application monitoring instrumentation API calls to your program source code",
	Version: AppVersion,
	Args:    cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			runInteractiveMode(cmd, args)
			return
		}
		cmd.Help()
	},
}

var debug bool

// usageOptOut declines anonymous usage reporting for this run. The flag name
// matches the diagnostics CLI's -usage-opt-out so that the two behave alike.
var usageOptOut bool

// interactiveMode records that the tool was driven interactively rather than
// invoked with a path.
var interactiveMode bool

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		cobra.CheckErr(err)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debugging output")
	rootCmd.PersistentFlags().BoolVar(&usageOptOut, "usage-opt-out", false, "decline to send anonymous usage data to New Relic for this run")
}
