// Copyright (C) 2015 Nippon Telegraph and Telephone Corporation.
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

	"github.com/spf13/cobra"

	"github.com/osrg/gobgp/v4/api"
	"github.com/osrg/gobgp/v4/internal/pkg/table"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

func showNetlink(args []string) error {
	res, err := client.GetNetlink(context.Background(), &api.GetNetlinkRequest{})
	if err != nil {
		return err
	}
	fmt.Printf("Import Netlink: %t\n", res.ImportEnabled)
	if res.ImportEnabled {
		fmt.Printf("  vrf: %s\n", res.Vrf)
		fmt.Printf("  interfaces: %s\n", res.Interfaces)
	}
	fmt.Printf("Export Netlink: %t\n", res.ExportEnabled)
	if res.ExportEnabled {
		fmt.Printf("  vrf: %s\n", res.Vrf)
		fmt.Printf("  community: %s\n", res.CommunityName)
		fmt.Printf("  community-list: %s\n", res.CommunityList)
		fmt.Printf("  large-community-list: %s\n", res.LargeCommunityList)
	}
	return nil
}

func enableNetlink(args []string, vrf string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: gobgp netlink enable [import|export] <arg>")
	}
	switch args[0] {
	case "import":
		if len(args) < 2 {
			return fmt.Errorf("usage: gobgp netlink enable import <ifname> [-vrf <vrf>]")
		}
		_, err := client.EnableNetlink(context.Background(), &api.EnableNetlinkRequest{
			Vrf:        vrf,
			Interfaces: args[1:],
		})
		return err
	case "export":
		if len(args) < 2 {
			return fmt.Errorf("usage: gobgp netlink enable export <community-name> | <community-list> | <large-community-list> [-vrf <vrf>]")
		}
		community := ""
		communityList := []string{}
		largeCommunityList := []string{}
		if _, err := table.ParseCommunity(args[1]); err == nil {
			community = args[1]
		} else if _, err := bgp.ParseLargeCommunity(args[1]); err == nil {
			largeCommunityList = append(largeCommunityList, args[1])
		} else {
			communityList = append(communityList, args[1])
		}
		_, err := client.EnableNetlink(context.Background(), &api.EnableNetlinkRequest{
			Vrf:                vrf,
			Community:          community,
			CommunityList:      communityList,
			LargeCommunityList: largeCommunityList,
		})
		return err
	default:
		return fmt.Errorf("unknown netlink type: %s", args[0])
	}
}

func newNetlinkCmd() *cobra.Command {
	var vrf string
	netlinkCmd := &cobra.Command{
		Use: "netlink",
		Run: func(cmd *cobra.Command, args []string) {
			showNetlink(args)
		},
	}

	enableCmd := &cobra.Command{
		Use: "enable",
		Run: func(cmd *cobra.Command, args []string) {
			if err := enableNetlink(args, vrf); err != nil {
				exitWithError(err)
			}
		},
	}
	enableCmd.PersistentFlags().StringVarP(&vrf, "vrf", "", "", "vrf")
	netlinkCmd.AddCommand(enableCmd)

	return netlinkCmd
}