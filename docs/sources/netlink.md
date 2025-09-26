# Netlink Integration

GoBGP can integrate with the Linux kernel's routing table using Netlink. This feature allows GoBGP to import routes from the kernel (e.g., connected routes) into its own RIB and export routes from its RIB into the kernel.

This is useful for scenarios where you want to advertise the host's own network interfaces via BGP or, conversely, have BGP-learned routes actively used by the host's networking stack.

## Configuration

Netlink integration can be configured via the GoBGP configuration file or the `gobgp` CLI.

### Configuration File

The following examples use the TOML format.

#### Importing Routes from the Kernel

To import routes from specific network interfaces into GoBGP's RIB, you can define a `[netlink.import]` section in your configuration file.

-   `enabled`: Must be `true` to enable the feature.
-   `vrf`: (Optional) The name of the VRF to import the routes into. If omitted, routes are imported into the global RIB.
-   `interface_list`: A list of interface names or glob patterns to match against. Routes from matching interfaces will be imported.

**Example:**

```toml
[netlink]
  [netlink.import]
    enabled = true
    vrf = "vrf-red"
    interface_list = ["eth0", "eth1", "vlan*"]
```

#### Exporting Routes to the Kernel

To export routes from GoBGP's RIB into the Linux kernel's routing table, you can define a `[netlink.export]` section.

-   `enabled`: Must be `true` to enable the feature.
-   `vrf`: (Optional) The name of the VRF from which to export routes. If omitted, routes are exported from the global RIB.
-   `community`: (Optional) A BGP community string. If specified, only routes tagged with this community will be exported.
-   `community_list`: (Optional) A list of BGP community strings.
-   `large_community_list`: (Optional) A list of BGP large community strings.

**Example:**

```toml
[netlink]
  [netlink.export]
    enabled = true
    vrf = "vrf-blue"
    community = "65001:100"
```

### CLI Configuration

You can also enable and manage netlink integration dynamically using the `gobgp netlink` command.

#### Enabling Import

To enable importing routes from one or more interfaces:

```bash
gobgp netlink enable import <interface_name> [interface_name...] [-vrf <vrf_name>]
```

-   `<interface_name>`: A space-separated list of interface names or glob patterns (e.g., `eth0`, `eth*`).
-   `-vrf`: (Optional) The VRF to import routes into.

**Example:**

```bash
gobgp netlink enable import eth0 eth1 -vrf vrf-red
```

#### Enabling Export

To enable exporting routes from GoBGP to the kernel:

```bash
gobgp netlink enable export [-vrf <vrf_name>] [--community <community>] [--community-list <communities>] [--large-community-list <large_communities>]
```

-   `-vrf`: (Optional) The VRF to export routes from.
-   `--community`: (Optional) Filter routes by a single community.
-   `--community-list`: (Optional) A comma-separated list of communities.
-   `--large-community-list`: (Optional) A comma-separated list of large communities.

**Example:**

```bash
gobgp netlink enable export --community 65001:100
```

## gRPC API

The Netlink feature can be controlled programmatically via the following gRPC RPCs.

### `EnableNetlink`

Enables import or export. The request can specify either interfaces for import or community filters for export.

**RPC Definition:**

```protobuf
rpc EnableNetlink(EnableNetlinkRequest) returns (EnableNetlinkResponse);
```

**Request Message:**

```protobuf
message EnableNetlinkRequest {
  // (Optional) The VRF name.
  string vrf = 1;
  // (Optional) A list of interface names or glob patterns for import.
  repeated string interfaces = 2;
  // (Optional) A single community for export filtering.
  string community_name = 3;
  // (Optional) A list of communities for export filtering.
  repeated string community_list = 4;
  // (Optional) A list of large communities for export filtering.
  repeated string large_community_list = 5;
}
```

### `GetNetlink`

Retrieves the current netlink configuration.

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
  string community_name = 5;
  repeated string community_list = 6;
  repeated string large_community_list = 7;
}
```
