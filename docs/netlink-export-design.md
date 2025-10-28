# Implement Policy-Based Route Export to Linux via Netlink

## Context & Motivation

### Current State
- ✅ **Import Complete**: Netlink import from Linux interfaces → BGP RIB (with VRF support) is fully implemented
- ✅ **VRF Support**: Per-VRF import configuration working
- ⚠️ **Export Needed**: Need to export BGP RIB routes back to Linux routing table
- 📊 **Policy System Available**: GoBGP has extensive 4500-line policy engine with community/prefix/AS-path filtering

### Why Policy-Based Approach?
Instead of simple community-based filtering in netlink code, integrate with GoBGP's existing policy engine because:
1. **Consistency**: Matches GoBGP's architecture (policies for import/export)
2. **Flexibility**: Supports complex filtering (prefix-lists, AS-path, multiple communities, etc.)
3. **Extensibility**: Easy to add new match conditions without modifying netlink code
4. **User Familiarity**: Users already know GoBGP policy syntax

### VRF Support Analysis
**Key Finding**: Policy system has NO direct per-VRF support, but:
- ✅ Global export policies exist (`global.apply-policy.config`)
- ✅ `CanImportToVrf()` uses Route Targets (RT) for VRF membership
- ✅ Can use community/RT matching in policies to filter per-VRF

**Solution**: New policy action that specifies target VRF and Linux table ID

---

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| **Architecture** | Policy-based with new `ACTION_EXPORT_TO_LINUX` | Leverage existing policy engine |
| **VRF Mapping** | Configuration-based (VRF name → table ID) | Explicit, predictable, no auto-discovery complexity |
| **Route Protocol** | `RTPROT_BGP` (186) | Standard Linux protocol for BGP routes |
| **Route Metric** | Configurable with default 20 | Allow priority tuning per deployment |
| **Multiple Matches** | Export to all matching Linux tables | Support multi-VRF scenarios |
| **Performance** | 1-second batching with rate limiting | Reduce netlink syscall overhead |
| **Withdrawal** | Auto-remove on BGP withdrawal | Keep Linux table synchronized |

---

## Implementation Phases

### Phase 1: Add NetlinkExportAction to Policy System
- [ ] Add `ACTION_EXPORT_TO_LINUX` to `ActionType` enum
- [ ] Implement `NetlinkExportAction` struct with `Apply()` method
- [ ] Add to statement actions list
- [ ] Extend OpenConfig types for configuration
- [ ] Files: `internal/pkg/table/policy.go`, `pkg/config/oc/bgp_configs.go`

### Phase 2: Implement netlinkExportClient with Batching
- [ ] Create `pkg/server/znetlink_export.go`
- [ ] Implement `netlinkExportClient` with route tracking map
- [ ] Add batch queue with 1-second ticker
- [ ] Implement `exportRoute()` and `withdrawRoute()` methods
- [ ] Add exported route tracking per-VRF
- [ ] Support metric configuration
- [ ] Files: New `pkg/server/znetlink_export.go`

### Phase 3: Hook into propagateUpdate() for RIB Changes
- [ ] Initialize `netlinkExportClient` in `BgpServer`
- [ ] Add export trigger after global export policy application
- [ ] Handle both new routes and withdrawals
- [ ] Add error handling and logging
- [ ] Files: `pkg/server/server.go`

### Phase 4: Add Configuration Parsing
- [ ] Extend `NetlinkExportActionConfig` in OpenConfig
- [ ] Add Linux table ID mapping to VRF config
- [ ] Parse metric configuration (default: 20)
- [ ] Support TOML/YAML/JSON/HCL formats
- [ ] Files: `pkg/config/oc/bgp_configs.go`, `pkg/config/config.go`

### Phase 5: Clean Up Old Export Stubs
- [ ] Remove `shouldExport()` stub from `znetlink.go`
- [ ] Remove incomplete `exportRoute()` from `znetlink.go`
- [ ] Update `NetlinkExport` struct usage (repurpose if needed)
- [ ] Files: `pkg/server/znetlink.go`

### Phase 6: Add gRPC/Protobuf Support
- [ ] Add `NetlinkExportAction` message to `gobgp.proto`
- [ ] Add `ListNetlinkExport` RPC
- [ ] Add `GetNetlinkExportStats` RPC
- [ ] Add `FlushNetlinkExport` RPC
- [ ] Implement gRPC handlers in `grpc_server.go`
- [ ] Regenerate protobuf code with `buf generate`
- [ ] Files: `proto/api/gobgp.proto`, `pkg/server/grpc_server.go`, `api/*.pb.go`

### Phase 7: Implement CLI Commands
- [ ] Add `gobgp netlink export` show commands
- [ ] Add `gobgp netlink export summary` statistics
- [ ] Add `gobgp netlink export flush` manual trigger
- [ ] Extend `gobgp policy` for netlink-export action
- [ ] Add enable/disable controls
- [ ] Files: `cmd/gobgp/netlink.go`, `cmd/gobgp/policy.go`

### Phase 8: Testing and Documentation
- [ ] Unit tests for `NetlinkExportAction`
- [ ] Unit tests for `netlinkExportClient`
- [ ] Integration tests with policy matching
- [ ] VRF export to Linux table validation
- [ ] Update `docs/netlink-export.md` (or create new)
- [ ] Add configuration examples
- [ ] Update `docs/policy.md` with new action type

---

## Technical Details

### New Action Type

```go
// internal/pkg/table/policy.go
const (
    ACTION_EXPORT_TO_LINUX ActionType = 9  // New action type
)

type NetlinkExportAction struct {
    VrfName      string
    TableId      int
    Metric       uint32
    exportClient *netlinkExportClient
}

func (a *NetlinkExportAction) Type() ActionType {
    return ACTION_EXPORT_TO_LINUX
}

func (a *NetlinkExportAction) Apply(path *Path, _ *PolicyOptions) (*Path, error) {
    if err := a.exportClient.queueExport(path, a.VrfName, a.TableId, a.Metric); err != nil {
        return path, err
    }
    return path, nil
}
```

### Export Client with Batching

```go
// pkg/server/znetlink_export.go
type netlinkExportClient struct {
    client          *netlink.NetlinkClient
    server          *BgpServer
    exportedRoutes  map[string]map[string]*go_netlink.Route  // vrf -> prefix -> route
    batchQueue      chan *exportJob
    batchTicker     *time.Ticker
    mu              sync.RWMutex
}

type exportJob struct {
    path    *table.Path
    vrfName string
    tableId int
    metric  uint32
}

func (e *netlinkExportClient) queueExport(path *table.Path, vrfName string, tableId int, metric uint32) error {
    select {
    case e.batchQueue <- &exportJob{path, vrfName, tableId, metric}:
        return nil
    default:
        return fmt.Errorf("export queue full")
    }
}

func (e *netlinkExportClient) startBatcher() {
    e.batchQueue = make(chan *exportJob, 1000)
    e.batchTicker = time.NewTicker(1 * time.Second)

    go func() {
        pending := make([]*exportJob, 0, 100)
        for {
            select {
            case job := <-e.batchQueue:
                pending = append(pending, job)
            case <-e.batchTicker.C:
                if len(pending) > 0 {
                    e.processBatch(pending)
                    pending = pending[:0]
                }
            }
        }
    }()
}

func (e *netlinkExportClient) processBatch(jobs []*exportJob) {
    for _, job := range jobs {
        if err := e.exportRoute(job); err != nil {
            e.server.logger.Error("Failed to export route",
                log.Fields{"prefix": job.path.GetNlri().String(), "error": err})
        }
    }
}

func (e *netlinkExportClient) exportRoute(job *exportJob) error {
    if job.path.IsWithdraw {
        return e.withdrawRoute(job.path, job.vrfName)
    }

    nlri := job.path.GetNlri()
    _, dst, err := net.ParseCIDR(nlri.String())
    if err != nil {
        return err
    }

    route := &go_netlink.Route{
        Dst:      dst,
        Gw:       job.path.GetNexthop().AsSlice(),
        Protocol: unix.RTPROT_BGP,
        Table:    job.tableId,
        Priority: int(job.metric),
    }

    if err := e.client.AddRoute(route); err != nil {
        return err
    }

    // Track for withdrawal
    e.mu.Lock()
    if e.exportedRoutes[job.vrfName] == nil {
        e.exportedRoutes[job.vrfName] = make(map[string]*go_netlink.Route)
    }
    e.exportedRoutes[job.vrfName][nlri.String()] = route
    e.mu.Unlock()

    return nil
}

func (e *netlinkExportClient) withdrawRoute(path *table.Path, vrfName string) error {
    e.mu.Lock()
    defer e.mu.Unlock()

    nlri := path.GetNlri()
    route, exists := e.exportedRoutes[vrfName][nlri.String()]
    if !exists {
        return nil
    }

    if err := e.client.DeleteRoute(route); err != nil {
        return err
    }

    delete(e.exportedRoutes[vrfName], nlri.String())
    return nil
}
```

---

## Configuration Examples

### Policy Definition with Export Action

```toml
# Define community set for filtering
[[defined-sets.bgp-defined-sets.community-sets]]
  community-set-name = "export-to-linux"
  community-list = ["65000:100", "65000:200"]

# Define policy with netlink export action
[[policy-definitions]]
  name = "export-customer-routes"
  [[policy-definitions.statements]]
    name = "match-export-community"
    [policy-definitions.statements.conditions.bgp-conditions.match-community-set]
      community-set = "export-to-linux"
      match-set-options = "any"
    [policy-definitions.statements.actions]
      route-disposition = "accept-route"
      [policy-definitions.statements.actions.netlink-export]
        enabled = true
        vrf = "customer-a"
        table-id = 100
        metric = 20

# Attach to global export policy
[global.apply-policy.config]
  export-policy-list = ["export-customer-routes"]
```

### VRF with Linux Table Mapping

```toml
[[vrfs]]
  [vrfs.config]
    name = "customer-a"
    rd = "64551:100"
    import-rt-list = ["64551:100"]
    export-rt-list = ["64551:100"]
  [vrfs.linux-table]
    table-id = 100  # Maps to Linux VRF routing table
```

### Multiple VRF Export Example

```toml
# Export customer-a routes (community 65000:100) to table 100
[[policy-definitions]]
  name = "export-customer-a"
  [[policy-definitions.statements]]
    [policy-definitions.statements.conditions.bgp-conditions.match-community-set]
      community-set = "customer-a-routes"
    [policy-definitions.statements.actions.netlink-export]
      vrf = "customer-a"
      table-id = 100

# Export customer-b routes (community 65000:200) to table 200
[[policy-definitions]]
  name = "export-customer-b"
  [[policy-definitions.statements]]
    [policy-definitions.statements.conditions.bgp-conditions.match-community-set]
      community-set = "customer-b-routes"
    [policy-definitions.statements.actions.netlink-export]
      vrf = "customer-b"
      table-id = 200

[global.apply-policy.config]
  export-policy-list = ["export-customer-a", "export-customer-b"]
```

---

## CLI Command Specifications

### Show Exported Routes
```bash
# Show all exported routes
$ gobgp netlink export
VRF          Prefix              Nexthop         Table  Metric  Status
global       10.1.0.0/24        192.168.1.1     254    20      exported
customer-a   10.2.0.0/24        192.168.2.1     100    20      exported
customer-a   10.3.0.0/24        192.168.2.1     100    20      failed

# Show specific VRF
$ gobgp netlink export --vrf customer-a

# JSON output
$ gobgp netlink export --json
```

### Statistics
```bash
$ gobgp netlink export summary
Export Statistics:
  Total routes exported: 245
  Export failures: 3
  By VRF:
    global: 120 routes
    customer-a: 85 routes
    customer-b: 40 routes
  Batch queue size: 12 pending
  Last batch: 2 seconds ago
```

### Manual Controls
```bash
# Force immediate export (bypass batching)
$ gobgp netlink export flush

# Disable/enable export globally
$ gobgp netlink export disable
$ gobgp netlink export enable
```

### Policy Configuration
```bash
# Add policy with netlink export action
$ gobgp policy add export-routes stmt1 \
  community export-to-linux \
  then netlink-export --vrf customer-a --table-id 100 --metric 20

# Show policy (should include netlink-export action)
$ gobgp policy show export-routes
```

---

## Protobuf API Definitions

```protobuf
// proto/api/gobgp.proto

// New action type for policy
message NetlinkExportAction {
  bool enabled = 1;
  string vrf = 2;
  int32 table_id = 3;
  uint32 metric = 4;
}

// Extend Statement message
message Statement {
  // ... existing fields ...
  NetlinkExportAction netlink_export = XX;  // Add to next available field number
}

// Exported route information
message ExportedRoute {
  string vrf = 1;
  string prefix = 2;
  string nexthop = 3;
  int32 table_id = 4;
  uint32 metric = 5;
  string status = 6;  // "exported", "failed", "pending"
  string error = 7;
  google.protobuf.Timestamp exported_at = 8;
}

// Statistics
message NetlinkExportStats {
  uint64 total_exported = 1;
  uint64 total_failed = 2;
  map<string, uint64> routes_by_vrf = 3;
  uint32 pending_batch_size = 4;
  google.protobuf.Timestamp last_batch = 5;
}

// RPCs
rpc ListNetlinkExport(ListNetlinkExportRequest) returns (stream ListNetlinkExportResponse);
rpc GetNetlinkExportStats(GetNetlinkExportStatsRequest) returns (GetNetlinkExportStatsResponse);
rpc FlushNetlinkExport(FlushNetlinkExportRequest) returns (FlushNetlinkExportResponse);
rpc EnableNetlinkExport(EnableNetlinkExportRequest) returns (EnableNetlinkExportResponse);
rpc DisableNetlinkExport(DisableNetlinkExportRequest) returns (DisableNetlinkExportResponse);

message ListNetlinkExportRequest {
  string vrf = 1;  // Filter by VRF (empty = all)
}

message ListNetlinkExportResponse {
  ExportedRoute route = 1;
}

message GetNetlinkExportStatsRequest {}

message GetNetlinkExportStatsResponse {
  NetlinkExportStats stats = 1;
}

message FlushNetlinkExportRequest {}

message FlushNetlinkExportResponse {
  uint32 flushed_count = 1;
}

message EnableNetlinkExportRequest {}
message EnableNetlinkExportResponse {}

message DisableNetlinkExportRequest {}
message DisableNetlinkExportResponse {}
```

---

## Files to Modify

### Core Implementation
- `internal/pkg/table/policy.go` - Add NetlinkExportAction
- `pkg/server/znetlink_export.go` - New file for export client
- `pkg/server/server.go` - Hook into propagateUpdate()
- `pkg/server/znetlink.go` - Remove old stubs

### Configuration
- `pkg/config/oc/bgp_configs.go` - Add NetlinkExportActionConfig, LinuxTable
- `pkg/config/config.go` - Parse new configuration

### API
- `proto/api/gobgp.proto` - Add messages and RPCs
- `api/*.pb.go` - Regenerate protobuf code
- `pkg/server/grpc_server.go` - Implement RPC handlers

### CLI
- `cmd/gobgp/netlink.go` - Add export subcommands
- `cmd/gobgp/policy.go` - Extend for netlink-export action

### Documentation
- `docs/netlink-export.md` - New documentation (or extend existing)
- `docs/policy.md` - Document new action type
- `docs/VRF-NETLINK-IMPORT.md` - Add export section

### Testing
- `internal/pkg/table/policy_test.go` - Test new action
- `pkg/server/znetlink_export_test.go` - Test export client
- `test/scenario_test/` - Add integration tests

---

## Testing Plan

### Unit Tests
1. **NetlinkExportAction.Apply()**
   - Test action execution
   - Test with withdraw paths
   - Test error handling

2. **netlinkExportClient**
   - Test route export
   - Test route withdrawal
   - Test batching mechanism
   - Test queue overflow handling
   - Test metric application

3. **Policy Integration**
   - Test policy parsing with netlink-export action
   - Test community matching → export
   - Test multiple policies with different VRFs

### Integration Tests
1. **Basic Export**
   - Create policy with export action
   - Add route with matching community
   - Verify route in Linux routing table
   - Verify correct protocol (RTPROT_BGP)
   - Verify correct metric

2. **VRF Export**
   - Configure VRF with Linux table mapping
   - Export routes to VRF table
   - Verify routes in correct Linux table

3. **Withdrawal**
   - Export route
   - Withdraw route from BGP
   - Verify route removed from Linux table

4. **Batching**
   - Rapid route updates
   - Verify batching occurs (1-second delay)
   - Verify all routes eventually exported

5. **Multiple VRFs**
   - Export same prefix to multiple VRFs
   - Verify in multiple Linux tables
   - Test with overlapping communities

### Manual Testing
- Test with real BGP sessions
- Verify Linux kernel routing table updates
- Test VRF routing with Linux VRF devices
- Performance testing with large route counts

---

## Success Criteria

- [ ] Routes with matching communities export to Linux routing table
- [ ] Export respects VRF configuration (correct table IDs)
- [ ] Withdrawal removes routes from Linux
- [ ] Batching reduces syscall overhead
- [ ] CLI commands show export status
- [ ] Configuration works in TOML/YAML/JSON/HCL
- [ ] Policy syntax matches existing GoBGP conventions
- [ ] No memory leaks or race conditions
- [ ] Performance acceptable (>1000 routes/sec)
- [ ] Documentation complete

---

## Related Work

- **Import Implementation**: Already complete (PR #XXX or commit XXX)
- **VRF Import**: Per-VRF netlink import working
- **Policy System**: Using existing 4500-line policy engine
- **Similar Feature**: Zebra integration (similar concept, different target)

---

## References

- [GoBGP Policy Documentation](https://github.com/osrg/gobgp/blob/master/docs/sources/policy.md)
- [Netlink Library Documentation](https://pkg.go.dev/github.com/vishvananda/netlink)
- [Linux VRF Documentation](https://www.kernel.org/doc/Documentation/networking/vrf.txt)
- [BGP Route Protocol (RTPROT_BGP)](https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/rtnetlink.h#L280)
