/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/cybrarymin/behavox/api"
	"github.com/spf13/cobra"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "showing the version and buildtime of the application",
	Long:  `showing the version and buildtime of the application`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("version: ", api.Version)
		fmt.Println("BuildTime: ", api.BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
