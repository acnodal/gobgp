# VRF-Aware Netlink Import

This document describes the VRF-aware netlink route import feature in GoBGP.

## Overview

GoBGP can import routes from Linux network interfaces via netlink and advertise them to BGP peers. This feature now supports importing routes into specific VRFs (Virtual Routing and Forwarding instances), enabling multi-tenant and segmented routing scenarios.

## Configuration

### Global Table Import

To import routes into the global BGP table (default behavior):

```toml
[netlink]
  [netlink.import]
    enabled = true
    interface-list = ["eth0", "eth1"]
```

### Per-VRF Import

To import routes into specific VRFs, configure netlink-import within each VRF definition:

```toml
[[vrfs]]
  [vrfs.config]
    name = "customer-a"
    rd = "64551:100"
    import-rt-list = ["64551:100"]
    export-rt-list = ["64551:100"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["eth1", "eth2"]

[[vrfs]]
  [vrfs.config]
    name = "customer-b"
    rd = "64551:200"
    import-rt-list = ["64551:200"]
    export-rt-list = ["64551:200"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["eth3"]
```

### Hybrid Configuration

You can combine global table import with per-VRF import:

```toml
# Import from kubelb0 into global table
[netlink]
  [netlink.import]
    enabled = true
    interface-list = ["kubelb0"]

# Import from eth1/eth2 into customer-a VRF
[[vrfs]]
  [vrfs.config]
    name = "customer-a"
    rd = "64551:100"
    import-rt-list = ["64551:100"]
    export-rt-list = ["64551:100"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["eth1", "eth2"]
```

## How It Works

1. **Route Discovery**: GoBGP periodically scans the configured interfaces for connected routes (every 5 seconds by default)

2. **VRF Assignment**: Routes are imported into the appropriate VRF based on which interface they're discovered on:
   - Routes from interfaces in `[netlink.import]` go to the global table (or the VRF specified in `[netlink.import.vrf]`)
   - Routes from interfaces in `[vrfs.netlink-import]` go to that specific VRF

3. **BGP Advertisement**: Imported routes are advertised to BGP peers with:
   - Route Distinguisher (RD) prepended for VRF routes
   - Route Target (RT) extended communities for VRF import/export
   - Nexthop set to the interface's IP address (with link-local for IPv6)

4. **Route Withdrawal**: When a route disappears from an interface, it's automatically withdrawn from BGP

## Path Tracking

The netlink client maintains separate path tracking for each VRF:
- **Global table**: Empty string key `""`
- **Named VRFs**: VRF name as key (e.g., `"customer-a"`)

This ensures that:
- Routes are correctly associated with their VRF
- Withdrawals only affect the appropriate VRF
- Multiple VRFs can have overlapping IP space

## Use Cases

### Multi-Tenant Service Provider

```toml
# Tenant A - e-commerce company
[[vrfs]]
  [vrfs.config]
    name = "tenant-a"
    rd = "64551:1000"
    import-rt-list = ["64551:1000"]
    export-rt-list = ["64551:1000"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["tenant-a-net0", "tenant-a-net1"]

# Tenant B - financial services
[[vrfs]]
  [vrfs.config]
    name = "tenant-b"
    rd = "64551:2000"
    import-rt-list = ["64551:2000"]
    export-rt-list = ["64551:2000"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["tenant-b-net0"]
```

### Edge Router with Customer Interfaces

```toml
# Customer 1 - 10.1.0.0/16
[[vrfs]]
  [vrfs.config]
    name = "cust1"
    rd = "65000:1"
    import-rt-list = ["65000:1"]
    export-rt-list = ["65000:1"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["eth1.100"]  # VLAN 100

# Customer 2 - 10.2.0.0/16
[[vrfs]]
  [vrfs.config]
    name = "cust2"
    rd = "65000:2"
    import-rt-list = ["65000:2"]
    export-rt-list = ["65000:2"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["eth1.200"]  # VLAN 200
```

### Kubernetes LoadBalancer with VRFs

```toml
# Production cluster services
[[vrfs]]
  [vrfs.config]
    name = "k8s-prod"
    rd = "64551:10"
    import-rt-list = ["64551:10"]
    export-rt-list = ["64551:10"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["kubelb-prod"]

# Staging cluster services
[[vrfs]]
  [vrfs.config]
    name = "k8s-staging"
    rd = "64551:20"
    import-rt-list = ["64551:20"]
    export-rt-list = ["64551:20"]
  [vrfs.netlink-import]
    enabled = true
    interface-list = ["kubelb-staging"]
```

## Limitations

1. **Interface Exclusivity**: Each interface can only be in one VRF's import list. If an interface appears in multiple configurations, routes will be imported into all matching VRFs (this may change in future versions).

2. **Linux VRF Devices**: This implementation uses configuration-based mapping. It does not automatically detect Linux kernel VRF devices. See future enhancements below.

3. **Route Overlaps**: If the same prefix exists in multiple VRFs via different interfaces, each VRF will have its own independent copy.

## Future Enhancements

### Linux VRF Device Detection

Potential future enhancement to automatically map routes to VRFs based on Linux kernel VRF membership:

```go
// Discover which Linux VRF a route belongs to
func getRouteVrf(route *netlink.Route) (string, error) {
    if route.Table > 0 && route.Table < 252 {
        vrf, err := client.GetVrfByTableId(route.Table)
        if err != nil {
            return "", err
        }
        return vrf.Name, nil
    }
    return "", nil  // Global table
}
```

This would allow automatic VRF detection without explicit interface configuration.

## Debugging

Enable debug logging to see netlink import activity:

```bash
./gobgpd -f config.toml --log-level=debug
```

Look for log messages with `Topic: "netlink"`:
- `"VRF": "<vrf-name>"` - Shows which VRF routes are imported into
- `"Interface": "<iface>"` - Shows which interface routes come from
- `"Route": "<prefix>"` - Shows the imported/withdrawn prefix

## Example Commands

### View Global RIB
```bash
./gobgp global rib
```

### View VRF RIB
```bash
./gobgp vrf customer-a rib
```

### List All VRFs
```bash
./gobgp vrf
```

### Monitor Route Changes
```bash
./gobgp monitor global rib
./gobgp monitor vrf customer-a rib
```

## See Also

- [Configuration Documentation](configuration.md)
- [VRF Documentation](evpn.md)
- [Netlink Import Documentation](netlink.md)
