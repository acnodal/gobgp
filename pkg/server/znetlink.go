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
	"net/netip"
	"time"

	"github.com/osrg/gobgp/v4/internal/pkg/table"
	"github.com/osrg/gobgp/v4/pkg/log"
	"github.com/osrg/gobgp/v4/pkg/netlink"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	custom_net "github.com/osrg/gobgp/v4/internal/pkg/netutils"
	go_netlink "github.com/vishvananda/netlink"
)

type netlinkClient struct {
	client          *netlink.NetlinkClient
	server          *BgpServer
	dead            chan struct{}
	// advertisedPaths tracks paths per VRF (vrf name -> prefix -> path)
	// empty string key is used for global table
	advertisedPaths map[string]map[string]*table.Path
}

func newNetlinkClient(s *BgpServer) (*netlinkClient, error) {
	s.logger.Debug("creating new netlink client", log.Fields{"Topic": "netlink"})
	n, err := netlink.NewNetlinkClient(s.logger)
	if err != nil {
		return nil, err
	}
	w := &netlinkClient{
		client:          n,
		server:          s,
		dead:            make(chan struct{}),
		advertisedPaths: make(map[string]map[string]*table.Path),
	}
	w.runImport()
	go w.loop()
	return w, nil
}

func (n *netlinkClient) runImport() {
	// Import for global table
	if n.server.bgpConfig.Netlink.Import.Enabled {
		vrfName := n.server.bgpConfig.Netlink.Import.Vrf
		interfaces := n.server.bgpConfig.Netlink.Import.InterfaceList
		n.importForVrf(vrfName, interfaces)
	}

	// Import for each VRF with netlink-import configured
	for _, vrf := range n.server.bgpConfig.Vrfs {
		if vrf.NetlinkImport.Enabled {
			n.importForVrf(vrf.Config.Name, vrf.NetlinkImport.InterfaceList)
		}
	}
}

func (n *netlinkClient) importForVrf(vrfName string, interfaces []string) {
	// Initialize VRF tracking if needed
	if n.advertisedPaths[vrfName] == nil {
		n.advertisedPaths[vrfName] = make(map[string]*table.Path)
	}

	currentPaths := make(map[string]*table.Path)

	// Scan interfaces for this VRF
	for _, iface := range interfaces {
		routes, err := custom_net.GetGlobalUnicastRoutes(iface, n.server.logger)
		if err != nil {
			n.server.logger.Error("failed to get connected routes",
				log.Fields{
					"Topic":     "netlink",
					"VRF":       vrfName,
					"Interface": iface,
					"Error":     err,
				})
			continue
		}
		for _, path := range n.ipNetsToPaths(routes, iface) {
			key := path.GetNlri().String()
			currentPaths[key] = path
		}
	}

	// Find new paths to add
	newPathList := make([]*table.Path, 0)
	for key, path := range currentPaths {
		if _, ok := n.advertisedPaths[vrfName][key]; !ok {
			newPathList = append(newPathList, path)
		}
	}

	// Find old paths to withdraw
	withdrawnPathList := make([]*table.Path, 0)
	for key, path := range n.advertisedPaths[vrfName] {
		if _, ok := currentPaths[key]; !ok {
			n.server.logger.Debug("Withdrawing route from netlink",
				log.Fields{
					"Topic": "netlink",
					"VRF":   vrfName,
					"Route": path.GetNlri().String(),
				})
			withdrawnPathList = append(withdrawnPathList, path.Clone(true))
		}
	}

	// Update advertised paths for this VRF
	n.advertisedPaths[vrfName] = currentPaths

	// Propagate changes
	if len(newPathList) > 0 {
		if err := n.server.addPathList(vrfName, newPathList); err != nil {
			n.server.logger.Error("failed to add path from netlink",
				log.Fields{
					"Topic": "netlink",
					"VRF":   vrfName,
					"Error": err,
				})
		}
	}

	if len(withdrawnPathList) > 0 {
		if err := n.server.addPathList(vrfName, withdrawnPathList); err != nil {
			n.server.logger.Error("failed to withdraw path from netlink",
				log.Fields{
					"Topic": "netlink",
					"VRF":   vrfName,
					"Error": err,
				})
		}
	}
}

func (n *netlinkClient) loop() {
	n.server.logger.Debug("starting netlink client loop", log.Fields{"Topic": "netlink"})
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.dead:
			return
		case <-ticker.C:
			n.runImport()
		}
	}
}

func (n *netlinkClient) ipNetsToPaths(routes []*custom_net.ConnectedRoute, iface string) []*table.Path {
	pathList := make([]*table.Path, 0, len(routes))
	for _, route := range routes {
		nlri, err := table.NewNlriFromAPI(route.Prefix)
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

		family := bgp.RF_IPv4_UC
		if route.Prefix.IP.To4() == nil {
			family = bgp.RF_IPv6_UC
			mpreach, _ := bgp.NewPathAttributeMpReachNLRI(family, []bgp.AddrPrefixInterface{nlri}, netip.MustParseAddr("::"))
			pattr = append(pattr, mpreach)
		} else {
			nexthop := bgp.NewPathAttributeNextHop("0.0.0.0")
			pattr = append(pattr, nexthop)
		}

		source := table.NewNetlinkPeerInfo(iface, n.server.logger)

		path := table.NewPath(family, source, nlri, false, pattr, time.Now(), false)
		path.SetIsFromExternal(true)
		pathList = append(pathList, path)
	}
	return pathList
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
	if !n.server.bgpConfig.Netlink.Export.Enabled {
		return false
	}
	// TODO: add community support
	return true
}