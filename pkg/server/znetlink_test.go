//go:build linux

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

package server

import (
	"context"
	"testing"

	"github.com/osrg/gobgp/v4/api"
	"github.com/stretchr/testify/assert"
)

func TestNetlinkClient(t *testing.T) {
	s := NewBgpServer()
	go s.Serve()

	err := s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        1,
			RouterId:   "1.1.1.1",
			ListenPort: -1,
		},
	})
	assert.NoError(t, err)

	_, err = newNetlinkClient(s)
	assert.NoError(t, err)
}

func TestEnableNetlink(t *testing.T) {
	s := NewBgpServer()
	go s.Serve()

	err := s.StartBgp(context.Background(), &api.StartBgpRequest{
		Global: &api.Global{
			Asn:        1,
			RouterId:   "1.1.1.1",
			ListenPort: -1,
		},
	})
	assert.NoError(t, err)

	// Test enabling import
	s.bgpConfig.Netlink.Import.Enabled = true
	s.bgpConfig.Netlink.Import.Vrf = "vrf1"
	s.bgpConfig.Netlink.Import.InterfaceList = []string{"eth0", "eth1"}
	err = s.StartNetlink(context.Background())
	assert.NoError(t, err)
	assert.True(t, s.bgpConfig.Netlink.Import.Enabled)
	assert.Equal(t, "vrf1", s.bgpConfig.Netlink.Import.Vrf)
	assert.Equal(t, []string{"eth0", "eth1"}, s.bgpConfig.Netlink.Import.InterfaceList)

	// Test enabling export with rules
	// Note: Export configuration now uses Rules structure instead of direct fields
	s.bgpConfig.Netlink.Export.Enabled = true
	err = s.StartNetlink(context.Background())
	assert.NoError(t, err)
	assert.True(t, s.bgpConfig.Netlink.Export.Enabled)
}
