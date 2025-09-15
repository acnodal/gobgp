// Copyright (C) 2025 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/osrg/gobgp/v4/api"
)

func showRedistribution(ctx context.Context, w io.Writer) error {
	res, err := client.GetRedistribution(ctx, &api.GetRedistributionRequest{})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "Import redistribution: %v\n", res.ImportEnabled)
	if res.ImportEnabled {
		fmt.Fprintf(w, "  vrf: %s\n", res.Vrf)
		fmt.Fprintf(w, "  interfaces: %v\n", res.Interfaces)
	}
	fmt.Fprintf(w, "Export redistribution: %v\n", res.ExportEnabled)
	if res.ExportEnabled {
		fmt.Fprintf(w, "  vrf: %s\n", res.Vrf)
		fmt.Fprintf(w, "  community-name: %s\n", res.CommunityName)
		fmt.Fprintf(w, "  community-list: %v\n", res.CommunityList)
		fmt.Fprintf(w, "  large-community-list: %v\n", res.LargeCommunityList)
	}
	return nil
}

func newRedistributionCmd() *cobra.Command {
	redistributionCmd := &cobra.Command{
		Use: "redistribution",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	enableCmd := &cobra.Command{
		Use: "enable",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	importCmd := &cobra.Command{
		Use: "import",
		Run: func(cmd *cobra.Command, args []string) {
			vrf, _ := cmd.Flags().GetString("vrf")
			_, err := client.EnableRedistribution(ctx, &api.EnableRedistributionRequest{
				Vrf:        vrf,
				Export:     false,
				Interfaces: args,
			})
			if err != nil {
				exitWithError(err)
			}
		},
	}
	importCmd.Flags().StringP("vrf", "v", "", "vrf name")

	exportCmd := &cobra.Command{
		Use: "export",
		Run: func(cmd *cobra.Command, args []string) {
			vrf, _ := cmd.Flags().GetString("vrf")
			community, _ := cmd.Flags().GetString("community")
			communityList, _ := cmd.Flags().GetStringSlice("community-list")
			largeCommunityList, _ := cmd.Flags().GetStringSlice("large-community-list")
			_, err := client.EnableRedistribution(ctx, &api.EnableRedistributionRequest{
				Vrf:                vrf,
				Export:             true,
				CommunityName:      community,
				CommunityList:      communityList,
				LargeCommunityList: largeCommunityList,
			})
			if err != nil {
				exitWithError(err)
			}
		},
	}
	exportCmd.Flags().StringP("vrf", "v", "", "vrf name")
	exportCmd.Flags().StringP("community", "c", "", "community name")
	exportCmd.Flags().StringSliceP("community-list", "l", []string{}, "community list")
	exportCmd.Flags().StringSliceP("large-community-list", "L", []string{}, "large community list")

	showCmd := &cobra.Command{
		Use: "show",
		Run: func(cmd *cobra.Command, args []string) {
			err := showRedistribution(ctx, cmd.OutOrStdout())
			if err != nil {
				exitWithError(err)
			}
		},
	}

	enableCmd.AddCommand(importCmd)
	enableCmd.AddCommand(exportCmd)
	redistributionCmd.AddCommand(enableCmd)
	redistributionCmd.AddCommand(showCmd)

	return redistributionCmd
}
