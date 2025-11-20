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
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/osrg/gobgp/v4/api"
)

func showNetlink() error {
	res, err := client.GetNetlink(context.Background(), &api.GetNetlinkRequest{})
	if err != nil {
		return err
	}

	fmt.Println("Netlink Status:")
	fmt.Println()

	fmt.Printf("Import: %t\n", res.ImportEnabled)
	if res.ImportEnabled {
		vrfDisplay := res.Vrf
		if vrfDisplay == "" {
			vrfDisplay = "global"
		}
		fmt.Printf("  VRF:        %s\n", vrfDisplay)
		fmt.Printf("  Interfaces: %s\n", res.Interfaces)
	}

	// Display VRF import configurations
	if len(res.VrfImports) > 0 {
		fmt.Println()
		fmt.Println("VRF Imports:")
		for _, vrfImport := range res.VrfImports {
			fmt.Printf("  VRF:        %s\n", vrfImport.VrfName)
			fmt.Printf("  Interfaces: %v\n", vrfImport.Interfaces)
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Printf("Export: %t\n", res.ExportEnabled)
	if res.ExportEnabled {
		fmt.Printf("  (use 'gobgp netlink export rules' to see export configuration)\n")
	}

	return nil
}

func showNetlinkImport() error {
	res, err := client.GetNetlink(context.Background(), &api.GetNetlinkRequest{})
	if err != nil {
		return err
	}

	hasImport := res.ImportEnabled || len(res.VrfImports) > 0

	if !hasImport {
		fmt.Println("Netlink import is not enabled")
		return nil
	}

	fmt.Println("Netlink Import Configuration:")
	fmt.Println()

	if res.ImportEnabled {
		vrfDisplay := res.Vrf
		if vrfDisplay == "" {
			vrfDisplay = "global"
		}
		fmt.Printf("Global Import:\n")
		fmt.Printf("  VRF:        %s\n", vrfDisplay)
		fmt.Printf("  Interfaces: %v\n", res.Interfaces)
		fmt.Println()
	}

	if len(res.VrfImports) > 0 {
		fmt.Println("VRF Imports:")
		for _, vrfImport := range res.VrfImports {
			fmt.Printf("  VRF:        %s\n", vrfImport.VrfName)
			fmt.Printf("  Interfaces: %v\n", vrfImport.Interfaces)
			fmt.Println()
		}
	}

	fmt.Println("Note: Imported routes are visible in the RIB")
	fmt.Println("      Use 'gobgp global rib' or 'gobgp vrf <name> rib' to view imported routes")

	return nil
}

func showNetlinkImportStats() error {
	res, err := client.GetNetlinkImportStats(context.Background(), &api.GetNetlinkImportStatsRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("Import Statistics:\n")
	fmt.Printf("  Total Imported:  %d\n", res.Imported)
	fmt.Printf("  Total Withdrawn: %d\n", res.Withdrawn)
	fmt.Printf("  Total Errors:    %d\n", res.Errors)

	if res.LastImportTime > 0 {
		fmt.Printf("  Last Import:     %s\n", time.Unix(res.LastImportTime, 0).Format("2006-01-02 15:04:05"))
	}
	if res.LastWithdrawTime > 0 {
		fmt.Printf("  Last Withdraw:   %s\n", time.Unix(res.LastWithdrawTime, 0).Format("2006-01-02 15:04:05"))
	}
	if res.LastErrorTime > 0 {
		fmt.Printf("  Last Error:      %s\n", time.Unix(res.LastErrorTime, 0).Format("2006-01-02 15:04:05"))
		fmt.Printf("  Last Error Msg:  %s\n", res.LastErrorMsg)
	}

	return nil
}

func showNetlinkExport(vrf string) error {
	stream, err := client.ListNetlinkExport(context.Background(), &api.ListNetlinkExportRequest{
		Vrf: vrf,
	})
	if err != nil {
		return err
	}

	headerFormat := "%-40s %-20s %-15s %-8s %-6s %-20s %s\n"
	rowFormat := "%-40s %-20s %-15s %-8d %-6d %-20s %s\n"

	fmt.Printf(headerFormat, "Prefix", "Nexthop", "VRF", "Table ID", "Metric", "Rule", "Exported At")
	fmt.Printf(headerFormat, "------", "-------", "---", "--------", "------", "----", "-----------")

	for {
		r, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		route := r.Route
		vrfDisplay := route.Vrf
		if vrfDisplay == "" {
			vrfDisplay = "global"
		}

		exportedAt := time.Unix(route.ExportedAt, 0).Format("2006-01-02 15:04:05")

		fmt.Printf(rowFormat,
			route.Prefix,
			route.Nexthop,
			vrfDisplay,
			route.TableId,
			route.Metric,
			route.RuleName,
			exportedAt)
	}

	return nil
}

func showNetlinkExportRules() error {
	res, err := client.ListNetlinkExportRules(context.Background(), &api.ListNetlinkExportRulesRequest{})
	if err != nil {
		return err
	}

	if len(res.Rules) == 0 {
		fmt.Println("No export rules configured")
		return nil
	}

	for _, rule := range res.Rules {
		fmt.Printf("Rule: %s\n", rule.Name)

		vrfDisplay := rule.Vrf
		if vrfDisplay == "" {
			vrfDisplay = "global"
		}
		fmt.Printf("  VRF:              %s\n", vrfDisplay)
		fmt.Printf("  Table ID:         %d\n", rule.TableId)
		fmt.Printf("  Metric:           %d\n", rule.Metric)
		fmt.Printf("  Validate Nexthop: %t\n", rule.ValidateNexthop)

		if len(rule.CommunityList) > 0 {
			fmt.Printf("  Communities:      %s\n", rule.CommunityList[0])
			for _, comm := range rule.CommunityList[1:] {
				fmt.Printf("                    %s\n", comm)
			}
		}

		if len(rule.LargeCommunityList) > 0 {
			fmt.Printf("  Large Communities: %s\n", rule.LargeCommunityList[0])
			for _, lcomm := range rule.LargeCommunityList[1:] {
				fmt.Printf("                     %s\n", lcomm)
			}
		}

		if len(rule.CommunityList) == 0 && len(rule.LargeCommunityList) == 0 {
			fmt.Printf("  Communities:      (match all routes)\n")
		}

		fmt.Println()
	}

	// Display VRF export rules
	if len(res.VrfRules) > 0 {
		fmt.Println("Per-VRF Export Rules:")
		fmt.Println()

		for _, vrfRule := range res.VrfRules {
			fmt.Printf("VRF: %s → Linux VRF: %s\n", vrfRule.GobgpVrf, vrfRule.LinuxVrf)
			fmt.Printf("  Linux Table ID:   %d\n", vrfRule.LinuxTableId)
			fmt.Printf("  Metric:           %d\n", vrfRule.Metric)
			fmt.Printf("  Validate Nexthop: %t\n", vrfRule.ValidateNexthop)

			if len(vrfRule.CommunityList) > 0 {
				fmt.Printf("  Communities:      %s\n", vrfRule.CommunityList[0])
				for _, comm := range vrfRule.CommunityList[1:] {
					fmt.Printf("                    %s\n", comm)
				}
			}

			if len(vrfRule.LargeCommunityList) > 0 {
				fmt.Printf("  Large Communities: %s\n", vrfRule.LargeCommunityList[0])
				for _, lcomm := range vrfRule.LargeCommunityList[1:] {
					fmt.Printf("                     %s\n", lcomm)
				}
			}

			if len(vrfRule.CommunityList) == 0 && len(vrfRule.LargeCommunityList) == 0 {
				fmt.Printf("  Communities:      (match all routes)\n")
			}

			fmt.Println()
		}
	}

	return nil
}

func showNetlinkExportStats() error {
	res, err := client.GetNetlinkExportStats(context.Background(), &api.GetNetlinkExportStatsRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("Export Statistics:\n")
	fmt.Printf("  Total Exported:              %d\n", res.Exported)
	fmt.Printf("  Total Withdrawn:             %d\n", res.Withdrawn)
	fmt.Printf("  Total Errors:                %d\n", res.Errors)
	fmt.Printf("  Nexthop Validation Attempts: %d\n", res.NexthopValidationAttempts)
	fmt.Printf("  Nexthop Validation Failures: %d\n", res.NexthopValidationFailures)
	fmt.Printf("  Dampened Updates:            %d\n", res.DampenedUpdates)

	if res.LastExportTime > 0 {
		fmt.Printf("  Last Export:                 %s\n", time.Unix(res.LastExportTime, 0).Format("2006-01-02 15:04:05"))
	}
	if res.LastWithdrawTime > 0 {
		fmt.Printf("  Last Withdraw:               %s\n", time.Unix(res.LastWithdrawTime, 0).Format("2006-01-02 15:04:05"))
	}
	if res.LastErrorTime > 0 {
		fmt.Printf("  Last Error:                  %s\n", time.Unix(res.LastErrorTime, 0).Format("2006-01-02 15:04:05"))
		fmt.Printf("  Last Error Message:          %s\n", res.LastErrorMsg)
	}

	return nil
}

func flushNetlinkExport() error {
	_, err := client.FlushNetlinkExport(context.Background(), &api.FlushNetlinkExportRequest{})
	if err != nil {
		return err
	}
	fmt.Println("All exported routes flushed successfully")
	return nil
}

func newNetlinkCmd() *cobra.Command {
	var vrf string

	netlinkCmd := &cobra.Command{
		Use: "netlink",
		Run: func(cmd *cobra.Command, args []string) {
			if err := showNetlink(); err != nil {
				exitWithError(err)
			}
		},
	}

	// Import subcommand
	importCmd := &cobra.Command{
		Use: "import",
		Run: func(cmd *cobra.Command, args []string) {
			if err := showNetlinkImport(); err != nil {
				exitWithError(err)
			}
		},
	}

	// Import stats subcommand
	importStatsCmd := &cobra.Command{
		Use: "stats",
		Run: func(cmd *cobra.Command, args []string) {
			if err := showNetlinkImportStats(); err != nil {
				exitWithError(err)
			}
		},
	}
	importCmd.AddCommand(importStatsCmd)

	netlinkCmd.AddCommand(importCmd)

	// Export subcommand
	exportCmd := &cobra.Command{
		Use: "export",
		Run: func(cmd *cobra.Command, args []string) {
			if err := showNetlinkExport(vrf); err != nil {
				exitWithError(err)
			}
		},
	}
	exportCmd.PersistentFlags().StringVarP(&vrf, "vrf", "", "", "filter by vrf name")

	// Export rules subcommand
	exportRulesCmd := &cobra.Command{
		Use: "rules",
		Run: func(cmd *cobra.Command, args []string) {
			if err := showNetlinkExportRules(); err != nil {
				exitWithError(err)
			}
		},
	}
	exportCmd.AddCommand(exportRulesCmd)

	// Export stats subcommand
	exportStatsCmd := &cobra.Command{
		Use: "stats",
		Run: func(cmd *cobra.Command, args []string) {
			if err := showNetlinkExportStats(); err != nil {
				exitWithError(err)
			}
		},
	}
	exportCmd.AddCommand(exportStatsCmd)

	// Export flush subcommand
	exportFlushCmd := &cobra.Command{
		Use: "flush",
		Run: func(cmd *cobra.Command, args []string) {
			if err := flushNetlinkExport(); err != nil {
				exitWithError(err)
			}
		},
	}
	exportCmd.AddCommand(exportFlushCmd)

	netlinkCmd.AddCommand(exportCmd)

	return netlinkCmd
}