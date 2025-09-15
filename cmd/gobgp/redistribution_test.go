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
	"bytes"
	"context"
	"testing"

	"github.com/osrg/gobgp/v4/api"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func Test_Redistribution(t *testing.T) {
	assert := assert.New(t)

	// mock server
	s := newMockServer()
	defer s.stop()

	// replace the global client with the mock client
	client = s.client
	ctx = context.Background()

	// show
	s.setNextResponse(&api.GetRedistributionResponse{
		ImportEnabled:      true,
		Vrf:                "vrf1",
		Interfaces:         []string{"eth0", "eth1"},
		ExportEnabled:      true,
		CommunityName:      "test",
		CommunityList:      []string{"100:100"},
		LargeCommunityList: []string{"200:200:200"},
	}, nil)
	rootCmd := &cobra.Command{
		Use: "gobgp",
	}
	rootCmd.AddCommand(newRedistributionCmd())
	buf := bytes.NewBuffer(make([]byte, 0, 1024))
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"redistribution", "show"})
	err := rootCmd.Execute()
	assert.Nil(err)
	assert.Contains(buf.String(), "Import redistribution: true")
	assert.Contains(buf.String(), "Export redistribution: true")

	// enable import
	s.setNextResponse(&api.EnableRedistributionResponse{}, nil)
	rootCmd.SetArgs([]string{"redistribution", "enable", "import", "eth0", "-v", "vrf1"})
	err = rootCmd.Execute()
	assert.Nil(err)

	// enable export
	s.setNextResponse(&api.EnableRedistributionResponse{}, nil)
	rootCmd.SetArgs([]string{"redistribution", "enable", "export", "test", "-v", "vrf1"})
	err = rootCmd.Execute()
	assert.Nil(err)
}
