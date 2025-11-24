# Netlink Nexthop Fix Plan

## Problem Summary

During the merge from upstream/master (commit 17a1bdb3), critical netlink-specific nexthop handling logic was lost, causing incorrect nexthop behavior for netlink-imported routes.

### Current Broken Behavior
- IPv6 routes sent to IPv4 peers: Missing link-local nexthop (only global set, or incorrect)
- IPv4 routes sent to IPv6 peers: Wrong nexthop family
- All netlink routes treated same as regular BGP routes (nexthop-self logic applies incorrectly)

### Root Cause
1. **Lost functionality:**
   - `SetNexthops()` function (plural) that could set both global and link-local IPv6 nexthops
   - `path.GetSource().IsNetlink` check to distinguish netlink routes from regular BGP routes
   - Special handling for netlink routes in both eBGP and iBGP cases
   - `bgp.NewPathAttributeMpReachNLRIwithNexthops()` function (removed upstream, replaced with variadic `NewPathAttributeMpReachNLRI(...nextHops)`)

2. **Incorrect attempted fixes:**
   - Lines 248-255 in path.go: Incorrect fix that doesn't check IsNetlink and doesn't include link-local

## Design Principles

1. **Netlink routes use nexthop-self**: Always set nexthop to peer's interface addresses for netlink-originated routes
2. **IPv6 requires global + link-local**: For IPv6 routes, include both global and link-local nexthops from peer's interface
3. **Cross-family support**: IPv4 peer can advertise IPv6 routes (and vice versa) if interface has both address families
4. **Non-netlink routes**: Keep upstream default BGP behavior (no special handling)
5. **Defensive programming**: Check for nil nexthops and log warnings when missing

## Implementation Plan

### Step 1: Re-implement `SetNexthops()` function in path.go

**Location:** [internal/pkg/table/path.go](internal/pkg/table/path.go) after `SetNexthop()` (around line 481)

**Implementation:**
```go
func (path *Path) SetNexthops(nexthops []net.IP) {
	if len(nexthops) == 0 {
		return
	}

	// Convert net.IP to netip.Addr for new upstream API
	nextHopAddrs := make([]netip.Addr, 0, len(nexthops))
	for _, nh := range nexthops {
		if addr, ok := netip.AddrFromSlice(nh); ok {
			nextHopAddrs = append(nextHopAddrs, addr)
		}
	}

	if len(nextHopAddrs) == 0 {
		return
	}

	// Handle IPv4 routes with IPv6 nexthops (RFC 5549 - Extended Next Hop)
	if path.GetFamily() == bgp.RF_IPv4_UC && nexthops[0].To4() == nil {
		path.delPathAttr(bgp.BGP_ATTR_TYPE_NEXT_HOP)
		mpreach, _ := bgp.NewPathAttributeMpReachNLRI(path.GetFamily(),
			[]bgp.PathNLRI{{NLRI: path.GetNlri(), ID: path.localID}},
			nextHopAddrs...)
		path.setPathAttr(mpreach)
		return
	}

	// Handle traditional NEXT_HOP attribute (IPv4 routes with IPv4 nexthop)
	attr := path.getPathAttr(bgp.BGP_ATTR_TYPE_NEXT_HOP)
	if attr != nil {
		pa, _ := bgp.NewPathAttributeNextHop(nextHopAddrs[0])
		path.setPathAttr(pa)
	}

	// Handle MP_REACH_NLRI attribute (IPv6 routes, VPN routes, etc.)
	attr = path.getPathAttr(bgp.BGP_ATTR_TYPE_MP_REACH_NLRI)
	if attr != nil {
		oldNlri := attr.(*bgp.PathAttributeMpReachNLRI)
		// Use variadic args - supports both single and dual nexthops
		mpreach, _ := bgp.NewPathAttributeMpReachNLRI(path.GetFamily(),
			oldNlri.Value,
			nextHopAddrs...)
		path.setPathAttr(mpreach)
	}
}
```

### Step 2: Restore netlink-specific logic in `UpdatePathAttrs()` for eBGP

**Location:** [internal/pkg/table/path.go:246-277](internal/pkg/table/path.go#L246-L277)

**Changes:**
1. Remove incorrect fix (lines 248-255)
2. Replace with netlink-aware logic

**Implementation:**
```go
	}

	localAddress := info.LocalAddress
	nexthop := path.GetNexthop()

	switch peer.State.PeerType {
	case oc.PEER_TYPE_EXTERNAL:
		// Nexthop handling for netlink-originated routes
		if path.GetSource().IsNetlink {
			family := path.GetFamily()
			if family == bgp.RF_IPv6_UC {
				// IPv6 routes: use global + link-local nexthops from peer's interface
				if info.IPv6Nexthop != nil && !info.IPv6Nexthop.IsUnspecified() {
					nexthops := []net.IP{info.IPv6Nexthop}
					if info.IPv6LinkLocalNexthop != nil && !info.IPv6LinkLocalNexthop.IsUnspecified() {
						nexthops = append(nexthops, info.IPv6LinkLocalNexthop)
					}
					path.SetNexthops(nexthops)
				} else {
					logger.Warn("could not determine a valid IPv6 nexthop for netlink-originated route",
						slog.String("Topic", "Peer"),
						slog.String("Key", peer.State.NeighborAddress),
						slog.String("Prefix", path.GetPrefix().String()))
				}
			} else if family == bgp.RF_IPv4_UC {
				// IPv4 routes: use IPv4 nexthop from peer's interface
				if info.IPv4Nexthop != nil && !info.IPv4Nexthop.IsUnspecified() {
					path.SetNexthop(info.IPv4Nexthop)
				} else {
					logger.Warn("could not determine a valid IPv4 nexthop for netlink-originated route",
						slog.String("Topic", "Peer"),
						slog.String("Key", peer.State.NeighborAddress),
						slog.String("Prefix", path.GetPrefix().String()))
				}
			}
		} else {
			// Non-netlink routes: keep upstream default behavior
			if !path.IsLocal() || nexthop.IsUnspecified() {
				path.SetNexthop(localAddress.AsSlice())
			}
		}

		// remove-private-as handling
		path.RemovePrivateAS(peer.Config.LocalAs, peer.State.RemovePrivateAs)

		// ... rest of eBGP handling remains unchanged
```

### Step 3: Restore netlink-specific logic in `UpdatePathAttrs()` for iBGP

**Location:** [internal/pkg/table/path.go:278-293](internal/pkg/table/path.go#L278-L293)

**Implementation:**
```go
	case oc.PEER_TYPE_INTERNAL:
		// Nexthop handling for netlink-originated routes
		// For netlink routes, always set nexthop (treat as locally-originated)
		if path.GetSource().IsNetlink {
			family := path.GetFamily()
			if family == bgp.RF_IPv6_UC {
				// IPv6 routes: use global + link-local nexthops from peer's interface
				if info.IPv6Nexthop != nil && !info.IPv6Nexthop.IsUnspecified() {
					nexthops := []net.IP{info.IPv6Nexthop}
					if info.IPv6LinkLocalNexthop != nil && !info.IPv6LinkLocalNexthop.IsUnspecified() {
						nexthops = append(nexthops, info.IPv6LinkLocalNexthop)
					}
					path.SetNexthops(nexthops)
				} else {
					logger.Warn("could not determine a valid IPv6 nexthop for netlink-originated route",
						slog.String("Topic", "Peer"),
						slog.String("Key", peer.State.NeighborAddress),
						slog.String("Prefix", path.GetPrefix().String()))
				}
			} else if family == bgp.RF_IPv4_UC {
				// IPv4 routes: use IPv4 nexthop from peer's interface
				if info.IPv4Nexthop != nil && !info.IPv4Nexthop.IsUnspecified() {
					path.SetNexthop(info.IPv4Nexthop)
				} else {
					logger.Warn("could not determine a valid IPv4 nexthop for netlink-originated route",
						slog.String("Topic", "Peer"),
						slog.String("Key", peer.State.NeighborAddress),
						slog.String("Prefix", path.GetPrefix().String()))
				}
			}
		} else {
			// Non-netlink routes: keep upstream default iBGP behavior
			if path.IsLocal() && nexthop.IsUnspecified() {
				path.SetNexthop(localAddress.AsSlice())
			}
		}

		// ... rest of iBGP handling remains unchanged
```

### Step 4: Add enhanced logging in server.go

**Location:** [pkg/server/server.go:1593](pkg/server/server.go#L1593)

**Implementation:**
After the IPv6 nexthop population block (line 1593), add:

```go
			}

			// Log warning if cross-family nexthops couldn't be populated
			if localAddr.Is4() && interfaceName != "" {
				// IPv4 session - check if we got IPv6 nexthops
				if peer.peerInfo.IPv6Nexthop == nil {
					s.logger.Warn("Could not populate IPv6 nexthop for IPv4 peer - IPv6 routes may fail",
						slog.String("Topic", "Peer"),
						slog.String("Key", peer.ID()),
						slog.String("Interface", interfaceName))
				}
			}
			if localAddr.Is6() && !localAddr.Is4In6() && interfaceName != "" {
				// IPv6 session - check if we got IPv4 nexthops
				if peer.peerInfo.IPv4Nexthop == nil {
					s.logger.Warn("Could not populate IPv4 nexthop for IPv6 peer - IPv4 routes may fail",
						slog.String("Topic", "Peer"),
						slog.String("Key", peer.ID()),
						slog.String("Interface", interfaceName))
				}
			}

			deferralExpiredFunc := func(family bgp.Family) func() {
```

## Verification Strategy

### Pre-implementation checks
1. Verify server.go lines 1552-1593 correctly populate cross-family nexthops ✓ (already verified)
2. Confirm `path.GetSource().IsNetlink` is available and returns correct value
3. Check PeerInfo has IPv4Nexthop, IPv6Nexthop, IPv6LinkLocalNexthop fields ✓ (already verified)

### Post-implementation testing
1. **Build verification**: Ensure code compiles without errors
2. **Single IPv4 peer test**:
   - IPv4 routes → peer receives with IPv4 nexthop
   - IPv6 routes → peer receives with IPv6 global + link-local nexthops
3. **Single IPv6 peer test**:
   - IPv6 routes → peer receives with IPv6 global + link-local nexthops
   - IPv4 routes → peer receives with IPv4 nexthop
4. **Dual peer test** (currently broken):
   - IPv4 peer + IPv6 peer simultaneously
   - Both receive correct nexthops for both route families
5. **VRF test**:
   - VRF with import interface + BGP neighbor
   - Routes use peer's interface nexthops (nexthop-self)
6. **Logging verification**:
   - Check warning logs appear when nexthops are missing
   - Verify peer establishment logs show populated nexthops

## Files Modified

1. `internal/pkg/table/path.go`:
   - Add `SetNexthops()` function (~line 481)
   - Modify `UpdatePathAttrs()` eBGP case (~line 246-277)
   - Modify `UpdatePathAttrs()` iBGP case (~line 278-293)

2. `pkg/server/server.go`:
   - Add warning logs after nexthop population (~line 1593)

## Expected Behavior After Fix

### For netlink-imported routes advertised to BGP peers:

**IPv4 Peer (10.0.0.1) receiving routes:**
- IPv4 route (192.168.1.0/24): Nexthop = 10.0.0.2 (peer's IPv4 interface address)
- IPv6 route (2001:db8::/64): Nexthop = [2001:db8::1, fe80::1] (peer's IPv6 global + link-local)

**IPv6 Peer (2001:db8::10) receiving routes:**
- IPv4 route (192.168.1.0/24): Nexthop = 10.0.0.2 (peer's IPv4 interface address)
- IPv6 route (2001:db8::/64): Nexthop = [2001:db8::1, fe80::1] (peer's IPv6 global + link-local)

**VRF Peer on vrf-peer0 interface:**
- All routes: Nexthop = addresses from vrf-peer0 interface (nexthop-self)

### For regular BGP routes:
- Behavior unchanged from upstream (standard BGP nexthop processing)

## Notes

- This fix restores carefully-designed pre-merge functionality while adapting to new upstream API
- The key insight: netlink routes are locally-originated, so nexthop-self always applies
- Cross-family routing requires peer's interface to have both IPv4 and IPv6 addresses
- VRF scenarios work correctly because peer's PeerInfo uses the VRF peering interface addresses
