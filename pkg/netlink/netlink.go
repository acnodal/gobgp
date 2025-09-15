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

type NetlinkClient struct {
	logger log.Logger
}

func NewNetlinkClient(logger log.Logger) (*NetlinkClient, error) {
	return &NetlinkClient{
		logger: logger,
	}, nil
}

func (n *NetlinkClient) GetConnectedRoutes(vrf string) ([]*netlink.Route, error) {
	var family int
	if vrf != "" {
		link, err := netlink.LinkByName(vrf)
		if err != nil {
			return nil, err
		}
		switch link.Type() {
		case "device":
			family = netlink.FAMILY_V4
		default:
			family = netlink.FAMILY_ALL
		}
	} else {
		family = netlink.FAMILY_ALL
	}
	routes, err := netlink.RouteList(nil, family)
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
	return netlink.RouteAdd(route)
}