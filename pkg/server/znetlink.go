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
	"net"
	"time"

	"github.com/osrg/gobgp/v4/internal/pkg/table"
	"github.com/osrg/gobgp/v4/pkg/log"
	"github.com/osrg/gobgp/v4/pkg/netlink"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	go_netlink "github.com/vishvananda/netlink"
)

type netlinkClient struct {
	client *netlink.NetlinkClient
	server *BgpServer
	dead   chan struct{}
	vrf    string
}

func newNetlinkClient(s *BgpServer, vrf string) (*netlinkClient, error) {
	n, err := netlink.NewNetlinkClient(s.logger)
	if err != nil {
		return nil, err
	}
	w := &netlinkClient{
		client: n,
		server: s,
		dead:   make(chan struct{}),
		vrf:    vrf,
	}
	go w.loop()
	return w, nil
}

func (n *netlinkClient) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.dead:
			return
		case <-ticker.C:
			routes, err := n.client.GetConnectedRoutes(n.vrf)
			if err != nil {
				n.server.logger.Error("failed to get connected routes",
					log.Fields{
						"Topic": "netlink",
						"Error": err,
					})
				continue
			}
			pathList := n.netlinkRoutesToPaths(routes)
			if err := n.server.addPathList(n.vrf, pathList); err != nil {
				n.server.logger.Error("failed to add path from netlink",
					log.Fields{
						"Topic": "netlink",
						"Error": err,
					})
			}
		}
	}
}

func (n *netlinkClient) netlinkRoutesToPaths(routes []*go_netlink.Route) []*table.Path {
	pathList := make([]*table.Path, 0, len(routes))
	for _, route := range routes {
		if route.Dst == nil {
			continue
		}

		if !n.shouldRedistribute(route) {
			continue
		}

		nlri, err := table.NewNlriFromAPI(route.Dst)
		if err != nil {
			n.server.logger.Warn("failed to create nlri from netlink route",
				log.Fields{
					"Topic": "netlink",
					"Route": route,
					"Error": err,
				})
			continue
		}

		pattr := make([]bgp.PathAttributeInterface, 0)
		origin := bgp.NewPathAttributeOrigin(bgp.BGP_ORIGIN_ATTR_TYPE_IGP)
		pattr = append(pattr, origin)

		if route.Gw != nil {
			nexthop := bgp.NewPathAttributeNextHop(route.Gw.String())
			pattr = append(pattr, nexthop)
		} else {
			if len(route.Dst.IP) > 0 {
				nexthop := bgp.NewPathAttributeNextHop(route.Dst.IP.String())
				pattr = append(pattr, nexthop)
			}
		}
		family := bgp.RF_IPv4_UC
		if route.Dst.IP.To4() == nil {
			family = bgp.RF_IPv6_UC
		}

		path := table.NewPath(family, nil, nlri, false, pattr, time.Now(), false)
		path.SetIsFromExternal(true)
		pathList = append(pathList, path)
	}
	return pathList
}

func (n *netlinkClient) shouldRedistribute(route *go_netlink.Route) bool {
	if !n.server.bgpConfig.Netlink.Config.Enabled {
		return false
	}
	interfaces := n.server.bgpConfig.Netlink.Config.InterfaceList
	if len(interfaces) == 0 {
		return true
	}

	link, err := go_netlink.LinkByIndex(route.LinkIndex)
	if err != nil {
		return false
	}

	for _, iface := range interfaces {
		if iface == link.Attrs().Name {
			return true
		}
	}
	return false
}

func (n *netlinkClient) exportRoute(path *table.Path) error {
	if path.IsWithdraw {
		// TODO: handle withdraw
		return nil
	}

	if !n.shouldExport(path) {
		return nil
	}

	nlri := path.GetNlri()
	if nlri == nil {
		return nil
	}

	_, dst, err := net.ParseCIDR(nlri.String())
	if err != nil {
		return err
	}

	route := &go_netlink.Route{
		Dst:      dst,
		Gw:       path.GetNexthop().AsSlice(),
		Protocol: 2, // kernel
	}

	return n.client.AddRoute(route)
}

func (n *netlinkClient) shouldExport(path *table.Path) bool {
	if !n.server.bgpConfig.Netlink.Config.Enabled {
		return false
	}
	// TODO: add community support
	return true
}