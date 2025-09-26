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

package netlink

import (
	"net"
	"testing"

	"github.com/osrg/gobgp/v4/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

type mockNetlinkManager struct {
	routes    []netlink.Route
	added     *netlink.Route
	link      netlink.Link
	linkErr   error
	routeErr  error
	addErr    error
	linkbynameErr error
}

func (m *mockNetlinkManager) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return m.routes, m.routeErr
}

func (m *mockNetlinkManager) RouteAdd(route *netlink.Route) error {
	m.added = route
	return m.addErr
}

func (m *mockNetlinkManager) LinkByName(name string) (netlink.Link, error) {
	return m.link, m.linkbynameErr
}

func TestNewNetlinkClient(t *testing.T) {
	logger := log.NewDefaultLogger()
	_, err := NewNetlinkClient(logger)
	assert.NoError(t, err)
}

func TestGetConnectedRoutes(t *testing.T) {
	logger := log.NewDefaultLogger()
	client, _ := NewNetlinkClient(logger)

	_, ipNet, _ := net.ParseCIDR("192.168.1.0/24")
	mockManager := &mockNetlinkManager{
		routes: []netlink.Route{
			{
				Dst:      ipNet,
				Protocol: 2, // kernel
			},
			{
				Dst:      ipNet,
				Protocol: 3, // boot
			},
		},
	}
	client.manager = mockManager

	routes, err := client.GetConnectedRoutes("")
	assert.NoError(t, err)
	assert.Len(t, routes, 1)
	assert.Equal(t, netlink.RouteProtocol(2), routes[0].Protocol)
}

func TestAddRoute(t *testing.T) {
	logger := log.NewDefaultLogger()
	client, _ := NewNetlinkClient(logger)

	mockManager := &mockNetlinkManager{}
	client.manager = mockManager

	_, ipNet, _ := net.ParseCIDR("10.0.0.0/24")
	route := &netlink.Route{
		Dst: ipNet,
	}
	err := client.AddRoute(route)
	assert.NoError(t, err)
	assert.Equal(t, route, mockManager.added)
}
