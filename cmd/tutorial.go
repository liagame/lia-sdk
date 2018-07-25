package cmd

import (
	"fmt"
	"github.com/liagame/lia-cli/config"
	"github.com/liagame/lia-cli/internal"
	"github.com/spf13/cobra"
	"os"
	"strconv"
)

var debugMode bool

var tutorialCmd = &cobra.Command{
	Use:   "tutorial <number> <botDir>",
	Short: "Runs tutorial specified by number with chosen bot",
	Long:  `Runs tutorial specified by number with chosen bot`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		internal.UpdateIfTime(true)

		tutorialNumber, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to convert %s to number.\n %s\n", args[0], err)
			os.Exit(config.Generic)
		}
		botDir := args[1]

		internal.Tutorial(tutorialNumber, botDir, debugMode)
	},
}

func init() {
	rootCmd.AddCommand(tutorialCmd)

	tutorialCmd.Flags().BoolVarP(&debugMode, "debug", "d", false, "Toggle if you want to manually run your bot (eg. "+
		"through debug mode in IDE)")
}