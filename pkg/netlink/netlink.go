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
	"github.com/osrg/gobgp/v4/pkg/log"
	"github.com/vishvananda/netlink"
)

type NetlinkManager interface {
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	RouteAdd(route *netlink.Route) error
	LinkByName(name string) (netlink.Link, error)
}

type DefaultNetlinkManager struct{}

func (m *DefaultNetlinkManager) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	return netlink.RouteList(link, family)
}

func (m *DefaultNetlinkManager) RouteAdd(route *netlink.Route) error {
	return netlink.RouteAdd(route)
}

func (m *DefaultNetlinkManager) LinkByName(name string) (netlink.Link, error) {
	return netlink.LinkByName(name)
}

type NetlinkClient struct {
	logger  log.Logger
	manager NetlinkManager
}

func NewNetlinkClient(logger log.Logger) (*NetlinkClient, error) {
	return &NetlinkClient{
		logger:  logger,
		manager: &DefaultNetlinkManager{},
	}, nil
}

func (n *NetlinkClient) GetConnectedRoutes(interfaceName string) ([]*netlink.Route, error) {
	link, err := n.manager.LinkByName(interfaceName)
	if err != nil {
		return nil, err
	}

	routes, err := n.manager.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		return nil, err
	}

	connected := make([]*netlink.Route, 0)
	for i, route := range routes {
		if route.Protocol == 2 { // kernel
			connected = append(connected, &routes[i])
		}
	}
	return connected, nil
}

func (n *NetlinkClient) AddRoute(route *netlink.Route) error {
	return n.manager.RouteAdd(route)
}
