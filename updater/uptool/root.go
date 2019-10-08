package main

import (
	"os"
	"path/filepath"

	"github.com/safing/portbase/updater"
	"github.com/safing/portbase/utils"
	"github.com/spf13/cobra"
)

var registry *updater.ResourceRegistry

var rootCmd = &cobra.Command{
	Use:   "uptool",
	Short: "helper tool for the update process",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Usage()
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}

		registry = &updater.ResourceRegistry{}
		return registry.Initialize(utils.NewDirStructure(absPath, 0755))
	},
	SilenceUsage: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
