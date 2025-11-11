// Copyright (C) 2025 The GoBGP Authors.
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
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/osrg/gobgp/v4/internal/pkg/table"
	"github.com/osrg/gobgp/v4/pkg/log"
	"github.com/osrg/gobgp/v4/pkg/packet/bgp"

	go_netlink "github.com/vishvananda/netlink"
)

const (
	// RTPROT_BGP is the Linux route protocol for BGP routes
	RTPROT_BGP = 186

	// Default dampening interval to prevent flapping
	defaultDampeningInterval = 100 * time.Millisecond

	// Default metric for exported routes
	defaultMetric = 20
)

// exportRule defines a rule for exporting BGP routes to Linux routing tables
type exportRule struct {
	Name             string
	Communities      []uint32                // Standard communities (32-bit)
	LargeCommunities []*bgp.LargeCommunity  // Large communities (96-bit)
	VrfName          string                  // VRF name (empty = global table)
	TableId          int                     // Linux routing table ID
	Metric           uint32                  // Route metric
	ValidateNexthop  bool                    // Validate nexthop reachability (default: true)
}

// exportedRouteInfo tracks metadata about an exported route
type exportedRouteInfo struct {
	Route      *go_netlink.Route  // The Linux route that was installed
	RuleName   string              // Which export rule matched
	ExportedAt time.Time           // When the route was exported
}

// dampenEntry tracks pending route updates for dampening
type dampenEntry struct {
	path      *table.Path
	timer     *time.Timer
	updatedAt time.Time
}

// exportStats tracks export operation statistics
type exportStats struct {
	Exported          uint64    // Total routes exported
	Withdrawn         uint64    // Total routes withdrawn
	Errors            uint64    // Total errors
	NexthopValidation uint64    // Nexthop validation attempts
	NexthopFailed     uint64    // Nexthop validation failures
	DampenedUpdates   uint64    // Updates that were dampened
	LastExport        time.Time // Last successful export
	LastWithdraw      time.Time // Last successful withdrawal
	LastError         time.Time // Last error
	LastErrorMsg      string    // Last error message
}

// netlinkExportClient manages exporting BGP routes to Linux routing tables
type netlinkExportClient struct {
	client   *go_netlink.Handle
	server   *BgpServer
	logger   log.Logger
	rules    []*exportRule
	exported map[string]map[string]*exportedRouteInfo // vrf -> prefix -> info
	mu       sync.RWMutex

	// Dampening
	dampeningInterval time.Duration
	pendingUpdates    map[string]*dampenEntry // prefix -> entry
	dampenMu          sync.Mutex

	// Statistics
	stats exportStats
	statsMu sync.RWMutex

	// Route protocol
	routeProtocol int

	// Shutdown
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// newNetlinkExportClient creates a new netlink export client
func newNetlinkExportClient(server *BgpServer, logger log.Logger, routeProtocol int, dampeningInterval time.Duration) (*netlinkExportClient, error) {
	handle, err := go_netlink.NewHandle()
	if err != nil {
		return nil, fmt.Errorf("failed to create netlink handle: %w", err)
	}

	if routeProtocol == 0 {
		routeProtocol = RTPROT_BGP
	}

	if dampeningInterval == 0 {
		dampeningInterval = defaultDampeningInterval
	}

	client := &netlinkExportClient{
		client:            handle,
		server:            server,
		logger:            logger,
		rules:             make([]*exportRule, 0),
		exported:          make(map[string]map[string]*exportedRouteInfo),
		pendingUpdates:    make(map[string]*dampenEntry),
		routeProtocol:     routeProtocol,
		dampeningInterval: dampeningInterval,
		stopCh:            make(chan struct{}),
	}

	// Clean up any stale routes from previous runs
	if err := client.cleanupStaleRoutes(); err != nil {
		logger.Warn("Failed to cleanup stale routes at startup",
			log.Fields{
				"Topic": "netlink",
				"Error": err,
			})
	}

	return client, nil
}

// cleanupStaleRoutes removes any routes with our protocol that were left behind from previous runs
func (e *netlinkExportClient) cleanupStaleRoutes() error {
	e.logger.Info("Cleaning up stale netlink routes from previous runs",
		log.Fields{"Topic": "netlink", "Protocol": e.routeProtocol})

	// Get all routes from the main routing table
	routes, err := e.client.RouteList(nil, go_netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("failed to list routes: %w", err)
	}

	cleanedCount := 0
	for _, route := range routes {
		// Check if this route was created by us (matching our protocol)
		if route.Protocol == go_netlink.RouteProtocol(e.routeProtocol) {
			e.logger.Debug("Deleting stale route",
				log.Fields{
					"Topic":  "netlink",
					"Prefix": route.Dst.String(),
					"Table":  route.Table,
					"Metric": route.Priority,
				})

			if err := e.client.RouteDel(&route); err != nil {
				e.logger.Warn("Failed to delete stale route",
					log.Fields{
						"Topic":  "netlink",
						"Prefix": route.Dst.String(),
						"Error":  err,
					})
			} else {
				cleanedCount++
			}
		}
	}

	if cleanedCount > 0 {
		e.logger.Info("Cleaned up stale routes",
			log.Fields{
				"Topic": "netlink",
				"Count": cleanedCount,
			})
	}

	return nil
}

// addRule adds an export rule to the client
func (e *netlinkExportClient) addRule(rule *exportRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// setRules replaces all rules with a new set (for dynamic reconfiguration)
func (e *netlinkExportClient) setRules(rules []*exportRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = rules
}

// reEvaluateAllRoutes re-evaluates all routes in the RIB against new rules
// This should be called after rules are updated to ensure existing routes
// are exported/withdrawn according to the new rules
func (e *netlinkExportClient) reEvaluateAllRoutes(pathList []*table.Path) {
	e.logger.Info("Re-evaluating all routes with new export rules",
		log.Fields{
			"Topic":      "netlink",
			"PathCount":  len(pathList),
		})

	// Build a set of prefixes that should be exported based on new rules
	shouldExport := make(map[string]map[string]bool) // vrf -> prefix -> should export

	// Check each path against all rules
	for _, path := range pathList {
		if path.IsWithdraw {
			continue
		}

		prefix := path.GetNlri().String()

		// Check all rules
		e.mu.RLock()
		rules := e.rules
		e.mu.RUnlock()

		for _, rule := range rules {
			if e.matchesRule(path, rule) {
				vrfName := rule.VrfName
				if shouldExport[vrfName] == nil {
					shouldExport[vrfName] = make(map[string]bool)
				}
				shouldExport[vrfName][prefix] = true

				// Export the route (idempotency check inside exportRoute will prevent duplicates)
				e.exportRoute(path, rule)
			}
		}
	}

	// Now withdraw routes that are currently exported but no longer match any rule
	routesToWithdraw := make([]struct {
		vrf    string
		prefix string
		route  *go_netlink.Route
	}, 0)

	e.mu.RLock()
	for vrfName, vrfRoutes := range e.exported {
		for prefix, info := range vrfRoutes {
			// If this route is not in the shouldExport set, withdraw it
			if shouldExport[vrfName] == nil || !shouldExport[vrfName][prefix] {
				routesToWithdraw = append(routesToWithdraw, struct {
					vrf    string
					prefix string
					route  *go_netlink.Route
				}{vrfName, prefix, info.Route})
			}
		}
	}
	e.mu.RUnlock()

	// Withdraw routes outside the lock
	for _, w := range routesToWithdraw {
		e.logger.Info("Withdrawing route that no longer matches any rule",
			log.Fields{
				"Topic":  "netlink",
				"Prefix": w.prefix,
				"VRF":    w.vrf,
			})

		// Delete the route directly
		err := e.client.RouteDel(w.route)
		if err != nil {
			e.statsMu.Lock()
			e.stats.Errors++
			e.stats.LastError = time.Now()
			e.stats.LastErrorMsg = fmt.Sprintf("RouteDel failed for %s: %v", w.prefix, err)
			e.statsMu.Unlock()
			e.logger.Warn("Failed to withdraw route",
				log.Fields{
					"Topic":  "netlink",
					"Prefix": w.prefix,
					"VRF":    w.vrf,
					"Error":  err,
				})
		} else {
			// Remove from tracking
			e.mu.Lock()
			delete(e.exported[w.vrf], w.prefix)
			if len(e.exported[w.vrf]) == 0 {
				delete(e.exported, w.vrf)
			}
			e.mu.Unlock()

			e.statsMu.Lock()
			e.stats.Withdrawn++
			e.stats.LastWithdraw = time.Now()
			e.statsMu.Unlock()
		}
	}

	e.logger.Info("Route re-evaluation complete",
		log.Fields{
			"Topic": "netlink",
		})
}

// close shuts down the export client
func (e *netlinkExportClient) close() {
	close(e.stopCh)
	e.wg.Wait()
	if e.client != nil {
		e.client.Close()
	}
}

// matchesRule checks if a path matches an export rule's community filters
func (e *netlinkExportClient) matchesRule(path *table.Path, rule *exportRule) bool {
	// If no community filters specified, match all routes
	if len(rule.Communities) == 0 && len(rule.LargeCommunities) == 0 {
		return true
	}

	// Get communities from path
	communities := path.GetCommunities()
	largeCommunities := path.GetLargeCommunities()

	// Check standard communities (OR logic - match if path has ANY community from the list)
	if len(rule.Communities) > 0 {
		pathCommSet := make(map[uint32]bool)
		for _, comm := range communities {
			pathCommSet[comm] = true
		}

		// If path has ANY of the rule communities, it matches
		for _, ruleComm := range rule.Communities {
			if pathCommSet[ruleComm] {
				return true
			}
		}
	}

	// Check large communities (OR logic - match if path has ANY large community from the list)
	if len(rule.LargeCommunities) > 0 {
		pathLargeCommSet := make(map[string]bool)
		for _, lc := range largeCommunities {
			key := fmt.Sprintf("%d:%d:%d", lc.ASN, lc.LocalData1, lc.LocalData2)
			pathLargeCommSet[key] = true
		}

		// If path has ANY of the rule large communities, it matches
		for _, ruleLc := range rule.LargeCommunities {
			key := fmt.Sprintf("%d:%d:%d", ruleLc.ASN, ruleLc.LocalData1, ruleLc.LocalData2)
			if pathLargeCommSet[key] {
				return true
			}
		}
	}

	// No match found
	return false
}

// isNexthopReachable checks if a nexthop is reachable via the kernel routing table
func (e *netlinkExportClient) isNexthopReachable(nh net.IP, tableId int) bool {
	e.statsMu.Lock()
	e.stats.NexthopValidation++
	e.statsMu.Unlock()

	// Try to find a route to the nexthop
	routes, err := e.client.RouteGet(nh)
	if err != nil || len(routes) == 0 {
		e.statsMu.Lock()
		e.stats.NexthopFailed++
		e.statsMu.Unlock()
		return false
	}

	// If we're exporting to a specific table, verify the nexthop route is in that table
	if tableId > 0 {
		for _, route := range routes {
			if route.Table == tableId {
				return true
			}
		}
		// Nexthop not in target table
		e.statsMu.Lock()
		e.stats.NexthopFailed++
		e.statsMu.Unlock()
		return false
	}

	return true
}

// exportRoute exports a BGP path to the Linux routing table according to a rule
func (e *netlinkExportClient) exportRoute(path *table.Path, rule *exportRule) error {
	// Get prefix
	nlri := path.GetNlri()
	prefix := nlri.String()

	// Get nexthop
	nexthop := path.GetNexthop()
	if nexthop.IsUnspecified() {
		return fmt.Errorf("no valid nexthop for %s", prefix)
	}

	// Convert nexthop to net.IP
	nexthopIP := net.IP(nexthop.AsSlice())

	// Validate nexthop if enabled (default: true)
	if rule.ValidateNexthop {
		if !e.isNexthopReachable(nexthopIP, rule.TableId) {
			e.logger.Debug("Nexthop validation failed",
				log.Fields{
					"Topic":    "netlink",
					"Prefix":   prefix,
					"Nexthop":  nexthop.String(),
					"Rule":     rule.Name,
					"VRF":      rule.VrfName,
				})
			return fmt.Errorf("nexthop %s not reachable", nexthop.String())
		}
	}

	// Check if already exported (idempotency)
	e.mu.RLock()
	vrfRoutes, vrfExists := e.exported[rule.VrfName]
	if vrfExists {
		if existingInfo, exists := vrfRoutes[prefix]; exists {
			// Already exported - check if parameters changed
			if existingInfo.RuleName == rule.Name {
				// Same rule name - check if route parameters match
				existingRoute := existingInfo.Route
				if existingRoute.Table == rule.TableId &&
					existingRoute.Priority == int(rule.Metric) &&
					existingRoute.Gw.Equal(nexthopIP) {
					// Route already exported with exact same parameters
					e.mu.RUnlock()
					return nil
				}
				// Parameters changed, need to delete old route first
				e.mu.RUnlock()
				e.logger.Info("Route parameters changed, deleting old route before re-export",
					log.Fields{
						"Topic":       "netlink",
						"Prefix":      prefix,
						"Rule":        rule.Name,
						"OldMetric":   existingRoute.Priority,
						"NewMetric":   rule.Metric,
						"OldTable":    existingRoute.Table,
						"NewTable":    rule.TableId,
					})

				// Delete the old route
				if err := e.client.RouteDel(existingRoute); err != nil {
					e.logger.Warn("Failed to delete old route during parameter change",
						log.Fields{
							"Topic":  "netlink",
							"Prefix": prefix,
							"Error":  err,
						})
				}

				// Remove from tracking so we can add the new one
				e.mu.Lock()
				delete(e.exported[rule.VrfName], prefix)
				e.mu.Unlock()
				// Continue to add the new route below
			} else {
				e.mu.RUnlock()
			}
		} else {
			e.mu.RUnlock()
		}
	} else {
		e.mu.RUnlock()
	}

	// Create netlink route
	_, ipNet, err := net.ParseCIDR(prefix)
	if err != nil {
		return fmt.Errorf("failed to parse CIDR %s: %w", prefix, err)
	}

	route := &go_netlink.Route{
		Dst:      ipNet,
		Gw:       nexthopIP,
		Table:    rule.TableId,
		Priority: int(rule.Metric),
		Protocol: go_netlink.RouteProtocol(e.routeProtocol),
	}

	// Add the route
	err = e.client.RouteReplace(route)
	if err != nil {
		e.statsMu.Lock()
		e.stats.Errors++
		e.stats.LastError = time.Now()
		e.stats.LastErrorMsg = fmt.Sprintf("RouteReplace failed for %s: %v", prefix, err)
		e.statsMu.Unlock()

		e.logger.Warn("Failed to export route",
			log.Fields{
				"Topic":   "netlink",
				"Prefix":  prefix,
				"Nexthop": nexthop.String(),
				"Rule":    rule.Name,
				"VRF":     rule.VrfName,
				"Error":   err,
			})
		return fmt.Errorf("failed to add route %s: %w", prefix, err)
	}

	// Track exported route
	e.mu.Lock()
	if e.exported[rule.VrfName] == nil {
		e.exported[rule.VrfName] = make(map[string]*exportedRouteInfo)
	}
	e.exported[rule.VrfName][prefix] = &exportedRouteInfo{
		Route:      route,
		RuleName:   rule.Name,
		ExportedAt: time.Now(),
	}
	e.mu.Unlock()

	e.statsMu.Lock()
	e.stats.Exported++
	e.stats.LastExport = time.Now()
	e.statsMu.Unlock()

	e.logger.Info("Exported route to Linux",
		log.Fields{
			"Topic":   "netlink",
			"Prefix":  prefix,
			"Nexthop": nexthop.String(),
			"Rule":    rule.Name,
			"VRF":     rule.VrfName,
			"Table":   rule.TableId,
			"Metric":  rule.Metric,
		})

	return nil
}

// withdrawRoute removes a BGP path from the Linux routing table
func (e *netlinkExportClient) withdrawRoute(path *table.Path, vrfName string) error {
	// Get prefix
	nlri := path.GetNlri()
	prefix := nlri.String()

	// Check if this route was exported
	e.mu.RLock()
	vrfRoutes, vrfExists := e.exported[vrfName]
	if !vrfExists {
		e.mu.RUnlock()
		return nil // Not exported, nothing to do
	}

	info, exists := vrfRoutes[prefix]
	if !exists {
		e.mu.RUnlock()
		return nil // Not exported, nothing to do
	}
	route := info.Route
	e.mu.RUnlock()

	// Delete the route
	err := e.client.RouteDel(route)
	if err != nil {
		e.statsMu.Lock()
		e.stats.Errors++
		e.stats.LastError = time.Now()
		e.stats.LastErrorMsg = fmt.Sprintf("RouteDel failed for %s: %v", prefix, err)
		e.statsMu.Unlock()

		e.logger.Warn("Failed to withdraw route",
			log.Fields{
				"Topic":  "netlink",
				"Prefix": prefix,
				"VRF":    vrfName,
				"Error":  err,
			})
		return fmt.Errorf("failed to delete route %s: %w", prefix, err)
	}

	// Remove from tracking
	e.mu.Lock()
	delete(e.exported[vrfName], prefix)
	if len(e.exported[vrfName]) == 0 {
		delete(e.exported, vrfName)
	}
	e.mu.Unlock()

	e.statsMu.Lock()
	e.stats.Withdrawn++
	e.stats.LastWithdraw = time.Now()
	e.statsMu.Unlock()

	e.logger.Info("Withdrew route from Linux",
		log.Fields{
			"Topic":  "netlink",
			"Prefix": prefix,
			"VRF":    vrfName,
		})

	return nil
}

// processDampenedUpdate processes a route update after dampening delay
func (e *netlinkExportClient) processDampenedUpdate(path *table.Path) {
	nlri := path.GetNlri()
	prefix := nlri.String()

	e.dampenMu.Lock()
	delete(e.pendingUpdates, prefix)
	e.dampenMu.Unlock()

	// Process the update
	e.processUpdate(path)
}

// scheduleUpdate schedules a route update with dampening
func (e *netlinkExportClient) scheduleUpdate(path *table.Path) {
	if e.dampeningInterval == 0 {
		// No dampening, process immediately
		e.processUpdate(path)
		return
	}

	nlri := path.GetNlri()
	prefix := nlri.String()

	e.dampenMu.Lock()
	defer e.dampenMu.Unlock()

	// Check if there's already a pending update
	if entry, exists := e.pendingUpdates[prefix]; exists {
		// Cancel existing timer and create new one
		entry.timer.Stop()
		entry.path = path
		entry.updatedAt = time.Now()
		entry.timer = time.AfterFunc(e.dampeningInterval, func() {
			e.processDampenedUpdate(path)
		})
		e.statsMu.Lock()
		e.stats.DampenedUpdates++
		e.statsMu.Unlock()
	} else {
		// Create new dampening entry
		timer := time.AfterFunc(e.dampeningInterval, func() {
			e.processDampenedUpdate(path)
		})
		e.pendingUpdates[prefix] = &dampenEntry{
			path:      path,
			timer:     timer,
			updatedAt: time.Now(),
		}
	}
}

// processUpdate processes a route update (export or withdrawal)
func (e *netlinkExportClient) processUpdate(path *table.Path) {
	if path.IsWithdraw {
		// Withdraw from all VRFs where this route was exported
		nlri := path.GetNlri()
		prefix := nlri.String()

		e.mu.RLock()
		vrfsToWithdraw := make([]string, 0)
		for vrfName, vrfRoutes := range e.exported {
			if _, exists := vrfRoutes[prefix]; exists {
				vrfsToWithdraw = append(vrfsToWithdraw, vrfName)
			}
		}
		e.mu.RUnlock()

		for _, vrfName := range vrfsToWithdraw {
			e.withdrawRoute(path, vrfName)
		}
		return
	}

	// Check all rules to see if this path should be exported
	e.mu.RLock()
	rules := make([]*exportRule, len(e.rules))
	copy(rules, e.rules)
	e.mu.RUnlock()

	for _, rule := range rules {
		if e.matchesRule(path, rule) {
			e.exportRoute(path, rule)
		}
	}
}

// getStats returns current export statistics
func (e *netlinkExportClient) getStats() exportStats {
	e.statsMu.RLock()
	defer e.statsMu.RUnlock()
	return e.stats
}

// listExported returns all currently exported routes
func (e *netlinkExportClient) listExported() map[string]map[string]*exportedRouteInfo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Deep copy to avoid race conditions
	result := make(map[string]map[string]*exportedRouteInfo)
	for vrfName, vrfRoutes := range e.exported {
		result[vrfName] = make(map[string]*exportedRouteInfo)
		for prefix, info := range vrfRoutes {
			result[vrfName][prefix] = &exportedRouteInfo{
				Route:      info.Route,
				RuleName:   info.RuleName,
				ExportedAt: info.ExportedAt,
			}
		}
	}
	return result
}

// getRules returns a copy of the current export rules
func (e *netlinkExportClient) getRules() []*exportRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Deep copy to avoid race conditions
	result := make([]*exportRule, len(e.rules))
	for i, rule := range e.rules {
		// Copy the rule
		ruleCopy := &exportRule{
			Name:            rule.Name,
			Communities:     make([]uint32, len(rule.Communities)),
			LargeCommunities: make([]*bgp.LargeCommunity, len(rule.LargeCommunities)),
			VrfName:         rule.VrfName,
			TableId:         rule.TableId,
			Metric:          rule.Metric,
			ValidateNexthop: rule.ValidateNexthop,
		}
		copy(ruleCopy.Communities, rule.Communities)
		for j, lcomm := range rule.LargeCommunities {
			ruleCopy.LargeCommunities[j] = lcomm
		}
		result[i] = ruleCopy
	}
	return result
}

// flush removes all exported routes
func (e *netlinkExportClient) flush() error {
	e.mu.RLock()
	routesToDelete := make([]*go_netlink.Route, 0)
	for _, vrfRoutes := range e.exported {
		for _, info := range vrfRoutes {
			routesToDelete = append(routesToDelete, info.Route)
		}
	}
	e.mu.RUnlock()

	// Delete all routes
	for _, route := range routesToDelete {
		err := e.client.RouteDel(route)
		if err != nil {
			e.logger.Warn("Failed to delete route during flush",
				log.Fields{
					"Topic": "netlink",
					"Route": route.Dst.String(),
					"Error": err,
				})
		}
	}

	// Clear tracking
	e.mu.Lock()
	e.exported = make(map[string]map[string]*exportedRouteInfo)
	e.mu.Unlock()

	e.logger.Info("Flushed all exported routes",
		log.Fields{
			"Topic": "netlink",
			"Count": len(routesToDelete),
		})

	return nil
}
