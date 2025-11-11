# Netlink Integration

GoBGP can integrate with the Linux kernel's routing table using Netlink. This feature allows GoBGP to:
- **Import routes** from the kernel (e.g., connected routes) into its RIB
- **Export routes** from its RIB into the Linux kernel routing table

This is useful for scenarios where you want to advertise the host's own network interfaces via BGP or, conversely, have BGP-learned routes actively used by the host's networking stack.

## Table of Contents

1. [Netlink Import](#netlink-import)
2. [Netlink Export](#netlink-export)
3. [CLI Commands](#cli-commands)
4. [gRPC API](#grpc-api)

---

# Netlink Import

## Overview

Netlink import allows GoBGP to discover routes from Linux network interfaces and import them into the BGP RIB. This enables advertising connected routes (directly attached networks) via BGP without manual configuration.

## Features

- **Interface monitoring**: Continuously scans specified interfaces for route changes
- **VRF support**: Import routes into specific VRFs or the global RIB
- **Pattern matching**: Use glob patterns to match multiple interfaces (e.g., `eth*`, `vlan*`)
- **Automatic synchronization**: Routes are automatically added/withdrawn as interfaces change
- **Statistics tracking**: Monitor import operations, withdrawals, and errors

## Configuration

### Basic Configuration

```toml
[global.config]
  as = 65000
  router-id = "192.168.1.1"

[netlink.import]
  enabled = true
  vrf = ""  # Empty = global RIB
  interface-list = ["eth0", "eth1", "vlan*"]
```

### Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | false | Enable netlink import functionality |
| `vrf` | string | "" | VRF name to import routes into (empty = global RIB) |
| `interface-list` | []string | [] | List of interface names or glob patterns |

### Configuration Examples

#### Example 1: Import from Specific Interfaces

```toml
[netlink.import]
  enabled = true
  interface-list = ["eth0", "eth1"]
```

#### Example 2: Import with Glob Patterns

```toml
[netlink.import]
  enabled = true
  interface-list = ["vlan*", "eth*"]
```

#### Example 3: Import into VRF

```toml
[netlink.import]
  enabled = true
  vrf = "customer-a"
  interface-list = ["eth2", "eth3"]
```

## How Import Works

1. **Initialization**: On startup, GoBGP scans configured interfaces for existing routes
2. **Periodic Scanning**: Every 5 seconds, interfaces are rescanned for changes
3. **Route Import**: New routes are converted to BGP paths and added to the RIB
4. **Route Withdrawal**: Removed routes are withdrawn from the RIB
5. **BGP Advertisement**: Imported routes can be advertised to BGP peers based on policies

### Route Attributes

Imported routes are created with:
- **Origin**: IGP
- **Nexthop**: 0.0.0.0 for IPv4, :: for IPv6
- **Source**: Netlink peer (identified by interface name)
- **IsFromExternal**: true

## Verification

### Check Import Status

```bash
gobgp netlink import
```

**Example output:**
```
Netlink Import Configuration:
  VRF:        global
  Interfaces: [eth0 eth1]

Note: Imported routes are visible in the global RIB
      Use 'gobgp global rib' to view imported routes
```

### View Imported Routes

```bash
gobgp global rib
```

Routes imported from netlink will show the interface name as the source.

### View Import Statistics

```bash
gobgp netlink import stats
```

**Example output:**
```
Import Statistics:
  Total Imported:  42
  Total Withdrawn: 3
  Total Errors:    0
  Last Import:     2025-11-11 21:10:45
  Last Withdraw:   2025-11-11 20:58:12
```

---

# Netlink Export

## Overview

The netlink export feature allows GoBGP to export BGP routes from the RIB to the Linux kernel routing table using netlink. This enables BGP-learned routes to be used for actual packet forwarding by the Linux kernel.

## Key Features

- **Community-based filtering**: Export routes based on BGP standard communities (32-bit) and large communities (96-bit)
- **VRF support**: Export routes to specific Linux routing tables associated with VRFs
- **Multiple export rules**: Define multiple rules with different communities, tables, and metrics
- **Nexthop validation**: Verify nexthop reachability before exporting (enabled by default)
- **Route dampening**: Prevent flapping storms with configurable dampening interval (default: 100ms)
- **Automatic withdrawal**: Routes are automatically removed from Linux when withdrawn from BGP
- **Startup cleanup**: Stale routes from previous runs are cleaned up on startup
- **Statistics and monitoring**: Track export operations, errors, and nexthop validation
- **Multi-table support**: Single route can export to multiple tables if matching multiple rules

## Configuration

### Basic Configuration

```toml
[global.config]
  as = 65000
  router-id = "192.168.1.1"

[netlink.export]
  enabled = true
  dampening-interval = 100  # milliseconds (default: 100)
  route-protocol = 186      # RTPROT_BGP (default: 186)

  [[netlink.export.rules]]
    name = "export-customer-a"
    community-list = ["65000:100"]
    vrf = "customer-a"
    table-id = 100
    metric = 20
    validate-nexthop = true  # default: true
```

### Configuration Parameters

#### Global Export Settings

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | false | Enable netlink export functionality |
| `dampening-interval` | uint32 | 100 | Dampening interval in milliseconds to prevent flapping |
| `route-protocol` | int | 186 | Linux route protocol (RTPROT_BGP=186) |

#### Export Rule Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | Yes | Unique name for the export rule |
| `community-list` | []string | No | List of standard BGP communities (format: "AS:VALUE") |
| `large-community-list` | []string | No | List of large BGP communities (format: "ASN:LocalData1:LocalData2") |
| `vrf` | string | No | VRF name (empty = global routing table) |
| `table-id` | int | No | Linux routing table ID (0 = main table) |
| `metric` | uint32 | 20 | Route metric/priority in Linux routing table |
| `validate-nexthop` | bool | true | Validate nexthop reachability before exporting |

**Note**: If neither `community-list` nor `large-community-list` is specified, the rule matches ALL routes.

### Community Formats

**Standard Communities (32-bit):**
```toml
community-list = [
  "65000:100",      # AS:VALUE format
  "4259905636"      # Decimal format
]
```

**Large Communities (96-bit):**
```toml
large-community-list = [
  "65000:1:100",    # ASN:LocalData1:LocalData2
  "65000:2:200"
]
```

### Community Matching Logic

- Routes match a rule if they have **ANY** community from the list (OR logic)
- If a rule has no community filters, it matches **ALL** routes
- Routes can match multiple rules and be exported to multiple tables

## Configuration Examples

### Example 1: Simple Global Table Export

Export all routes with community 65000:100 to the main Linux routing table:

```toml
[netlink.export]
  enabled = true

  [[netlink.export.rules]]
    name = "export-to-main"
    community-list = ["65000:100"]
    metric = 20
```

### Example 2: Multi-VRF Setup

Export different routes to different VRF routing tables:

```toml
[netlink.export]
  enabled = true
  dampening-interval = 100

  [[netlink.export.rules]]
    name = "export-customer-a"
    community-list = ["65000:100"]
    vrf = "customer-a"
    table-id = 100
    metric = 20

  [[netlink.export.rules]]
    name = "export-customer-b"
    community-list = ["65000:200"]
    vrf = "customer-b"
    table-id = 200
    metric = 20
```

### Example 3: Large Communities

Use large communities for more granular control:

```toml
[netlink.export]
  enabled = true

  [[netlink.export.rules]]
    name = "export-service-routes"
    large-community-list = ["65000:1:100", "65000:1:101"]
    table-id = 300
    metric = 10
```

### Example 4: Export All Routes

Export all routes (no community filter):

```toml
[netlink.export]
  enabled = true

  [[netlink.export.rules]]
    name = "export-all"
    # No community-list = match all routes
    table-id = 100
    metric = 20
```

### Example 5: Disable Nexthop Validation

For specific use cases where nexthop validation should be skipped:

```toml
[netlink.export]
  enabled = true

  [[netlink.export.rules]]
    name = "export-without-validation"
    community-list = ["65000:999"]
    table-id = 400
    validate-nexthop = false  # Skip nexthop validation
```

### Example 6: Zebra/FRR Coexistence

Configure a different route protocol to coexist with Zebra/FRR:

```toml
[netlink.export]
  enabled = true
  route-protocol = 17  # RTPROT_ZEBRA uses 186, so use different value

  [[netlink.export.rules]]
    name = "export-bgp-routes"
    community-list = ["65000:100"]
    metric = 30  # Higher metric than Zebra routes
```

## Architecture

### Export Flow

1. **BGP Route Update**: Route is learned via BGP from a peer
2. **RIB Update**: Route is added to GoBGP's RIB via `rib.Update()`
3. **Export Trigger**: Export hook is called immediately after RIB update
4. **Rule Matching**: Route's communities are matched against export rules (OR logic)
5. **Dampening** (optional): Route update is delayed by dampening interval
6. **Nexthop Validation** (if enabled): Nexthop reachability is checked via `RouteGet()`
7. **Route Export**: Route is installed in Linux kernel via netlink `RouteReplace()`
8. **Tracking**: Route metadata is stored for idempotency and withdrawal

### Startup Cleanup

On startup, GoBGP:
1. Lists all routes in the Linux kernel
2. Deletes any routes matching the configured `route-protocol`
3. Starts with a clean slate before exporting new routes
4. This prevents stale routes from previous crashes/restarts

### Export Timing

**Critical Design Decision**: Routes are exported **AFTER** `rib.Update()` completes, ensuring:
- Route is present in the RIB before export
- Best path selection has occurred
- No race conditions with route processing

### Idempotency

The export client tracks all exported routes to ensure:
- Routes are not re-exported if already present with same parameters
- Parameter changes (metric, table-id) trigger delete + re-add
- Only changed routes trigger netlink syscalls
- Efficient operation at scale

### Route Withdrawal

When a BGP route is withdrawn:
1. Path withdrawal is detected in export hook
2. Route is looked up in export tracking map
3. Route is deleted from Linux kernel via `RouteDel()`
4. Tracking metadata is cleaned up

### Dynamic Configuration Reload

When config file changes are detected (SIGHUP):
1. New rules are parsed and compared to existing rules
2. If rules changed, all RIB routes are re-evaluated
3. Routes matching new rules are exported
4. Routes no longer matching are withdrawn
5. Routes with changed parameters (metric, table-id) are updated

## Advanced Topics

### Nexthop Validation

Nexthop validation ensures that routes are only exported if the nexthop is reachable:

**How it works:**
1. Before exporting, the client calls `RouteGet(nexthop)` via netlink
2. If a route to the nexthop exists, validation passes
3. For VRF exports, validation checks the nexthop route is in the target table
4. If validation fails, the route is NOT exported and statistics are updated

**When to disable:**
- Nexthops are known to be reachable via other mechanisms
- Performance is critical and validation overhead is too high
- Using route servers where nexthops may not be directly reachable

**Default behavior:** Enabled (recommended for most deployments)

### Dampening

Dampening prevents flapping routes from causing excessive kernel updates:

**How it works:**
1. When a route update occurs, a timer is started (default: 100ms)
2. If another update for the same prefix occurs within the interval, the timer is reset
3. When the timer expires, the final route state is exported
4. This coalesces rapid updates into a single kernel operation

**Configuration:**
```toml
dampening-interval = 100  # milliseconds
```

**Set to 0 to disable dampening** (immediate export on every update)

### Route Protocol Values

The `route-protocol` parameter sets the Linux route protocol identifier:

Common values:
- `186` (RTPROT_BGP) - Default, identifies routes as BGP-originated
- `11` (RTPROT_BIRD) - BIRD routing daemon
- `17` - Zebra/FRR (avoid conflicts)
- `2` (RTPROT_KERNEL) - Kernel routes

**Why it matters:**
- Allows distinguishing route sources
- Prevents conflicts with other routing daemons
- Enables selective route management
- Used for startup cleanup

### Multiple Rules and Multi-Table Export

A single BGP route can match multiple export rules and be exported to multiple tables:

```toml
[[netlink.export.rules]]
  name = "shared-services-vrf1"
  community-list = ["65000:999"]  # Shared service tag
  vrf = "vrf1"
  table-id = 100

[[netlink.export.rules]]
  name = "shared-services-vrf2"
  community-list = ["65000:999"]  # Same tag
  vrf = "vrf2"
  table-id = 200
```

A route with community `65000:999` will be exported to **both** table 100 and table 200.

---

# CLI Commands

## Netlink Status

### View Overall Status

```bash
gobgp netlink
```

**Example output:**
```
Netlink Status:

Import: true
  VRF:        global
  Interfaces: [eth0 eth1]

Export: true
  (use 'gobgp netlink export rules' to see export configuration)
```

## Import Commands

### View Import Configuration

```bash
gobgp netlink import
```

**Example output:**
```
Netlink Import Configuration:
  VRF:        global
  Interfaces: [eth0 eth1]

Note: Imported routes are visible in the global RIB
      Use 'gobgp global rib' to view imported routes
```

### View Import Statistics

```bash
gobgp netlink import stats
```

**Example output:**
```
Import Statistics:
  Total Imported:  42
  Total Withdrawn: 3
  Total Errors:    0
  Last Import:     2025-11-11 21:10:45
  Last Withdraw:   2025-11-11 20:58:12
```

## Export Commands

### View Exported Routes

```bash
# Show all exported routes
gobgp netlink export

# Filter by VRF
gobgp netlink export --vrf customer-a
```

**Example output:**
```
Prefix                                   Nexthop              VRF              Table ID Metric Rule                 Exported At
------                                   -------              ---              -------- ------ ----                 -----------
10.0.0.0/24                             192.168.1.1          customer-a       100      20     export-customer-a    2025-11-11 15:04:05
10.0.1.0/24                             192.168.1.1          customer-a       100      20     export-customer-a    2025-11-11 15:04:12
192.168.100.0/24                        10.0.0.1             customer-b       200      20     export-customer-b    2025-11-11 15:05:23
```

### View Export Rules

```bash
gobgp netlink export rules
```

**Example output:**
```
Rule: export-customer-a
  VRF:              customer-a
  Table ID:         100
  Metric:           20
  Validate Nexthop: true
  Communities:      65000:100
                    65000:101

Rule: export-customer-b
  VRF:              customer-b
  Table ID:         200
  Metric:           20
  Validate Nexthop: true
  Communities:      65000:200
```

### View Export Statistics

```bash
gobgp netlink export stats
```

**Example output:**
```
Export Statistics:
  Total Exported:              125
  Total Withdrawn:             23
  Total Errors:                2
  Nexthop Validation Attempts: 148
  Nexthop Validation Failures: 0
  Dampened Updates:            12
  Last Export:                 2025-11-11 15:04:05
  Last Withdraw:               2025-11-11 14:58:32
```

### Flush All Exported Routes

```bash
# Remove all exported routes from Linux routing tables
gobgp netlink export flush
```

## Command Structure

```
gobgp netlink              # Status overview
├── import                 # Import configuration
│   └── stats              # Import statistics
└── export                 # Exported routes
    ├── rules              # Export rules configuration
    ├── stats              # Export statistics
    └── flush              # Remove all exported routes
```

---

# gRPC API

## Import API

### GetNetlink

Retrieves the current netlink configuration (both import and export).

**RPC Definition:**
```protobuf
rpc GetNetlink(GetNetlinkRequest) returns (GetNetlinkResponse);
```

**Response Message:**
```protobuf
message GetNetlinkResponse {
  bool import_enabled = 1;
  bool export_enabled = 2;
  string vrf = 3;
  repeated string interfaces = 4;
}
```

### GetNetlinkImportStats

Get import statistics.

**RPC Definition:**
```protobuf
rpc GetNetlinkImportStats(GetNetlinkImportStatsRequest) returns (GetNetlinkImportStatsResponse);
```

**Response Message:**
```protobuf
message GetNetlinkImportStatsResponse {
  uint64 imported = 1;
  uint64 withdrawn = 2;
  uint64 errors = 3;
  int64 last_import_time = 4;
  int64 last_withdraw_time = 5;
  int64 last_error_time = 6;
  string last_error_msg = 7;
}
```

## Export API

### ListNetlinkExport

Stream exported routes.

**RPC Definition:**
```protobuf
rpc ListNetlinkExport(ListNetlinkExportRequest) returns (stream ListNetlinkExportResponse);
```

**Request Message:**
```protobuf
message ListNetlinkExportRequest {
  string vrf = 1; // Filter by VRF name (empty = all VRFs)
}
```

### ListNetlinkExportRules

Get configured export rules.

**RPC Definition:**
```protobuf
rpc ListNetlinkExportRules(ListNetlinkExportRulesRequest) returns (ListNetlinkExportRulesResponse);
```

**Response Message:**
```protobuf
message ListNetlinkExportRulesResponse {
  message ExportRule {
    string name = 1;
    repeated string community_list = 2;
    repeated string large_community_list = 3;
    string vrf = 4;
    int32 table_id = 5;
    uint32 metric = 6;
    bool validate_nexthop = 7;
  }
  repeated ExportRule rules = 1;
}
```

### GetNetlinkExportStats

Get export statistics.

**RPC Definition:**
```protobuf
rpc GetNetlinkExportStats(GetNetlinkExportStatsRequest) returns (GetNetlinkExportStatsResponse);
```

**Response Message:**
```protobuf
message GetNetlinkExportStatsResponse {
  uint64 exported = 1;
  uint64 withdrawn = 2;
  uint64 errors = 3;
  uint64 nexthop_validation_attempts = 4;
  uint64 nexthop_validation_failures = 5;
  uint64 dampened_updates = 6;
  int64 last_export_time = 7;
  int64 last_withdraw_time = 8;
  int64 last_error_time = 9;
  string last_error_msg = 10;
}
```

### FlushNetlinkExport

Remove all exported routes.

**RPC Definition:**
```protobuf
rpc FlushNetlinkExport(FlushNetlinkExportRequest) returns (FlushNetlinkExportResponse);
```

---

# Verification

## Verify Routes in Linux Kernel

```bash
# Check main routing table
ip route show

# Check specific table
ip route show table 100

# Check route details
ip route show table 100 10.0.0.0/24

# Show routes by protocol
ip route show proto 186

# Show all routes with details
ip route show table all
```

**Expected output:**
```
10.0.0.0/24 via 192.168.1.1 dev eth0 proto 186 metric 20
```

## Verify VRF Configuration

```bash
# List VRFs
ip vrf list

# Show routes in VRF
ip route show vrf customer-a

# Execute command in VRF context
ip vrf exec customer-a ip route show
```

## Monitor Route Changes

```bash
# Monitor route changes in real-time
ip monitor route

# Monitor specific table
ip monitor route table 100

# Monitor with protocol filter
watch -n 1 'ip route show proto 186'
```

---

# Troubleshooting

## Import Issues

### Routes Not Being Imported

**Check 1: Verify import is enabled**
```bash
gobgp netlink import
# Should show configuration
```

**Check 2: Verify interfaces exist**
```bash
ip link show
```

**Check 3: Check import statistics**
```bash
gobgp netlink import stats
# Look for errors
```

**Check 4: Check logs**
```bash
journalctl -u gobgpd -f | grep netlink
```

## Export Issues

### Routes Not Appearing in Linux

**Check 1: Verify export is enabled**
```bash
gobgp netlink
# Should show "Export: true"
```

**Check 2: Verify routes match community filters**
```bash
gobgp global rib -a ipv4 | grep <prefix>
# Check communities on the route
```

**Check 3: View export rules**
```bash
gobgp netlink export rules
# Verify community lists
```

**Check 4: Check export statistics**
```bash
gobgp netlink export stats
# Look for errors and nexthop validation failures
```

**Check 5: Check logs**
```bash
journalctl -u gobgpd -f | grep netlink
```

### Nexthop Validation Failures

If routes are not exporting due to nexthop validation:

```bash
# Check stats for validation failures
gobgp netlink export stats

# Verify nexthop is reachable
ping <nexthop-ip>

# Check routing table for nexthop route
ip route get <nexthop-ip>

# If nexthop validation is not needed, disable it in config
```

Update config:
```toml
[[netlink.export.rules]]
  validate-nexthop = false
```

### High Error Count

Check the last error message:
```bash
gobgp netlink export stats | grep "Last Error"
```

Common errors:
- **Permission denied**: gobgpd needs CAP_NET_ADMIN capability
- **Network unreachable**: Nexthop validation failed
- **Invalid argument**: Malformed route parameters

### Routes Not Updating on Config Change

```bash
# Send SIGHUP to reload config
kill -HUP $(pidof gobgpd)

# Check logs for config reload
journalctl -u gobgpd | grep "config changed"

# Verify new rules are loaded
gobgp netlink export rules
```

### Duplicate Routes

If you see duplicate routes from different sources:

```bash
# Check all route sources
ip route show table <table-id>

# Check protocol
ip route show proto 186

# Use different route protocol to avoid conflicts
```

---

# Performance Considerations

## Scale Limits

- **Import**: Can handle thousands of connected routes
- **Export**: Designed for large-scale deployments (tested with 100k+ routes)
- **Memory**: ~200 bytes per exported route
- **CPU**: Minimal overhead with dampening enabled

## Optimization Tips

### For Import
1. Use specific interface names instead of broad glob patterns
2. Import only necessary interfaces
3. Monitor import statistics for errors

### For Export
1. **Use dampening**: Prevents excessive kernel updates during route churn
2. **Selective export**: Use specific community filters to export only needed routes
3. **Disable nexthop validation** if not needed: Reduces overhead
4. **Monitor statistics**: Track nexthop validation and dampening

## Typical Performance

- Route export: ~10,000 routes/second
- Route withdrawal: ~15,000 routes/second
- Nexthop validation: ~5,000 validations/second

---

# Security Considerations

## Required Capabilities

GoBGPd requires the following Linux capabilities:
- `CAP_NET_ADMIN` - For netlink route manipulation

**Running with systemd:**
```ini
[Service]
AmbientCapabilities=CAP_NET_ADMIN
```

**Running with Docker:**
```bash
docker run --cap-add NET_ADMIN ...
```

## Best Practices

1. Use strict community filtering to control what gets exported
2. Enable nexthop validation (default) to prevent invalid routes
3. Monitor export statistics for unexpected behavior
4. Use route protocol identifier to distinguish GoBGP routes
5. Use VRF isolation for multi-tenant deployments

---

# Integration Examples

## Container/Kubernetes Environments

GoBGP can export routes for container networking:

```toml
[[netlink.export.rules]]
  name = "pod-network"
  community-list = ["65000:1000"]
  table-id = 0  # Main table
  metric = 10
```

## Service Mesh Integration

Export service mesh routes to Linux kernel:

```toml
[[netlink.export.rules]]
  name = "service-mesh-routes"
  large-community-list = ["65000:mesh:1"]
  table-id = 500
```

## Zebra/FRR Coexistence

Run GoBGP alongside Zebra/FRR:

```toml
[netlink.export]
  route-protocol = 17  # Use different protocol than Zebra's 186
```

---

# FAQ

**Q: Why are my imported routes not appearing in BGP RIB?**
A: Check that import is enabled, interfaces exist and are up, and review import statistics for errors.

**Q: Why are my routes not exporting?**
A: Check that routes have matching communities, nexthop validation is passing, and export is enabled.

**Q: Can I export the same route to multiple tables?**
A: Yes, create multiple rules with the same community filter but different table IDs.

**Q: What happens if GoBGP restarts?**
A: On startup, GoBGP cleans up any stale routes (matching route-protocol), then re-exports all routes based on current configuration.

**Q: How do I remove all exported routes?**
A: Use `gobgp netlink export flush` or restart GoBGP with export disabled.

**Q: Can I use both standard and large communities in the same rule?**
A: Yes, specify both `community-list` and `large-community-list`. The route matches if it has ANY community from either list.

**Q: What's the difference between metric and table-id?**
A: `table-id` is the Linux routing table number (like VRF). `metric` is the route priority within that table (lower = higher priority).

**Q: Does this work with IPv6?**
A: Yes, both IPv4 and IPv6 routes are supported for both import and export.

**Q: Can I export routes learned from specific peers only?**
A: Not directly. Use BGP policies to tag routes from specific peers with communities, then export based on those communities.

**Q: What happens during config reload?**
A: On SIGHUP, GoBGP re-evaluates all routes against new rules. Routes are added/removed/updated as needed.
