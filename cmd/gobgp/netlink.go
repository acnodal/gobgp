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
	"github.com/spf13/cobra"

	"github.com/osrg/gobgp/v4/api"
)

func newNetlinkCmd() *cobra.Command {
	netlinkCmd := &cobra.Command{
		Use: "netlink",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	enableCmd := &cobra.Command{
		Use: "enable",
		Run: func(cmd *cobra.Command, args []string) {
			vrf, _ := cmd.Flags().GetString("vrf")
			if _, err := client.EnableNetlink(ctx, &api.EnableNetlinkRequest{Enabled: true, Vrf: vrf, Interfaces: args}); err != nil {
				exitWithError(err)
			}
		},
	}
	enableCmd.Flags().StringP("vrf", "v", "", "vrf name")

	netlinkCmd.AddCommand(enableCmd)
	return netlinkCmd
}
