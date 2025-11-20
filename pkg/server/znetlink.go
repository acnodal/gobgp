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
	"net/netip"
	"sync"
	"time"

	"github.com/osrg/gobgp/v4/internal/pkg/table"
	"github.com/osrg/gobgp/v4/pkg/config/oc"
	"github.com/osrg/gobgp/v4/pkg/log"
	"github.com/osrg/gobgp/v4/pkg/netlink"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
	custom_net "github.com/osrg/gobgp/v4/internal/pkg/netutils"
)

type netlinkImportStats struct {
	Imported        uint64
	Withdrawn       uint64
	Errors          uint64
	LastImport      time.Time
	LastWithdraw    time.Time
	LastError       time.Time
	LastErrorMsg    string
}

type netlinkClient struct {
	client          *netlink.NetlinkClient
	server          *BgpServer
	dead            chan struct{}
	// advertisedPaths tracks paths per VRF (vrf name -> prefix -> path)
	// empty string key is used for global table
	advertisedPaths map[string]map[string]*table.Path
	stats           netlinkImportStats
	statsMu         sync.RWMutex
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
	// Count VRFs in both config and RIB for debugging
	configVrfCount := len(n.server.bgpConfig.Vrfs)
	ribVrfCount := 0
	if n.server.globalRib != nil {
		ribVrfCount = len(n.server.globalRib.Vrfs)
	}

	n.server.logger.Debug("Running netlink import",
		log.Fields{
			"Topic":         "netlink",
			"ConfigVRFs":    configVrfCount,
			"RibVRFs":       ribVrfCount,
		})

	// Import for global table
	if n.server.bgpConfig.Netlink.Import.Enabled {
		vrfName := n.server.bgpConfig.Netlink.Import.Vrf
		interfaces := n.server.bgpConfig.Netlink.Import.InterfaceList
		n.server.logger.Debug("Global netlink import enabled",
			log.Fields{
				"Topic":      "netlink",
				"TargetVRF":  vrfName,
				"Interfaces": interfaces,
			})
		n.importForVrf(vrfName, interfaces)
	}

	// Import for each VRF with netlink-import configured
	// Check both bgpConfig.Vrfs and globalRib.Vrfs since VRFs added via API
	// may not be in bgpConfig.Vrfs yet
	vrfConfigMap := make(map[string]*oc.Vrf)
	for i := range n.server.bgpConfig.Vrfs {
		vrf := &n.server.bgpConfig.Vrfs[i]
		vrfConfigMap[vrf.Config.Name] = vrf
	}

	// Iterate over active VRFs in the RIB
	if n.server.globalRib != nil {
		for vrfName := range n.server.globalRib.Vrfs {
			// Look up VRF config
			vrfConfig, hasConfig := vrfConfigMap[vrfName]

			n.server.logger.Debug("Checking VRF for import",
				log.Fields{
					"Topic":      "netlink",
					"VRF":        vrfName,
					"HasConfig":  hasConfig,
				})

			if hasConfig && vrfConfig.NetlinkImport.Enabled {
				n.server.logger.Info("Starting VRF import",
					log.Fields{
						"Topic":      "netlink",
						"VRF":        vrfName,
						"Interfaces": vrfConfig.NetlinkImport.InterfaceList,
					})
				n.importForVrf(vrfName, vrfConfig.NetlinkImport.InterfaceList)
			} else if hasConfig {
				n.server.logger.Debug("VRF import not enabled",
					log.Fields{
						"Topic":         "netlink",
						"VRF":           vrfName,
						"ImportEnabled": vrfConfig.NetlinkImport.Enabled,
					})
			} else {
				n.server.logger.Debug("No config found for VRF",
					log.Fields{
						"Topic": "netlink",
						"VRF":   vrfName,
					})
			}
		}
	}
}

func (n *netlinkClient) importForVrf(vrfName string, interfaces []string) {
	// Initialize VRF tracking if needed
	if n.advertisedPaths[vrfName] == nil {
		n.advertisedPaths[vrfName] = make(map[string]*table.Path)
	}

	n.server.logger.Debug("Starting VRF import scan",
		log.Fields{
			"Topic":      "netlink",
			"VRF":        vrfName,
			"Interfaces": interfaces,
		})

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
		n.server.logger.Debug("Found routes on interface",
			log.Fields{
				"Topic":      "netlink",
				"VRF":        vrfName,
				"Interface":  iface,
				"RouteCount": len(routes),
			})
		for _, path := range n.ipNetsToPaths(routes, iface) {
			key := path.GetNlri().String()
			currentPaths[key] = path
			n.server.logger.Debug("Adding route to current paths",
				log.Fields{
					"Topic":  "netlink",
					"VRF":    vrfName,
					"Route":  key,
					"Family": path.GetFamily().String(),
				})
		}
	}

	n.server.logger.Debug("VRF import scan complete",
		log.Fields{
			"Topic":           "netlink",
			"VRF":             vrfName,
			"CurrentPaths":    len(currentPaths),
			"AdvertisedPaths": len(n.advertisedPaths[vrfName]),
		})

	// Find new paths to add
	newPathList := make([]*table.Path, 0)
	for key, path := range currentPaths {
		if _, ok := n.advertisedPaths[vrfName][key]; !ok {
			newPathList = append(newPathList, path)
			n.server.logger.Debug("New route to import",
				log.Fields{
					"Topic":  "netlink",
					"VRF":    vrfName,
					"Route":  key,
					"Family": path.GetFamily().String(),
				})
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
		n.server.logger.Info("Importing routes to VRF",
			log.Fields{
				"Topic":      "netlink",
				"VRF":        vrfName,
				"RouteCount": len(newPathList),
			})
		if err := n.server.addPathList(vrfName, newPathList); err != nil {
			n.server.logger.Error("failed to add path from netlink",
				log.Fields{
					"Topic": "netlink",
					"VRF":   vrfName,
					"Error": err,
				})
			n.statsMu.Lock()
			n.stats.Errors++
			n.stats.LastError = time.Now()
			n.stats.LastErrorMsg = err.Error()
			n.statsMu.Unlock()
		} else {
			n.server.logger.Info("Successfully imported routes to VRF",
				log.Fields{
					"Topic":      "netlink",
					"VRF":        vrfName,
					"RouteCount": len(newPathList),
				})
			n.statsMu.Lock()
			n.stats.Imported += uint64(len(newPathList))
			n.stats.LastImport = time.Now()
			n.statsMu.Unlock()
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
			n.statsMu.Lock()
			n.stats.Errors++
			n.stats.LastError = time.Now()
			n.stats.LastErrorMsg = err.Error()
			n.statsMu.Unlock()
		} else {
			n.statsMu.Lock()
			n.stats.Withdrawn += uint64(len(withdrawnPathList))
			n.stats.LastWithdraw = time.Now()
			n.statsMu.Unlock()
		}
	}
}

// rescan triggers an immediate import scan (called when VRFs are added/changed)
func (n *netlinkClient) rescan() {
	n.server.logger.Debug("Triggering immediate netlink import rescan", log.Fields{"Topic": "netlink"})
	n.runImport()
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

// getStats returns a copy of the current import statistics
func (n *netlinkClient) getStats() netlinkImportStats {
	n.statsMu.RLock()
	defer n.statsMu.RUnlock()
	return n.stats
}