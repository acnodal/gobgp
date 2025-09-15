# Project: Add redistribute to and from gobgp to host using netlink


You are an export in Linux Networking, go programming and network routing




# Overview.
gobgp is used to accept and export routes using bgp.  Its RIB does not interact with linux networking by default.  The purpose of this project is to add support to enable routing information to be "redistributed" between the hosts linux network routing and the gobgp RIB.  This is generally refered to as *redistribute connected* in other routing software.


# Features

* add redistribution functionality allowing gobgp to read all connected routes or routes from a list of specific interfaces
* add redistribution functionality distribute gobgp routes (RIB) to be inserted into the linux routing table
* add additional capability to distribute routes from the gobgp RIB based upon matching bgp communities
* add the ability to configure the new redistribution capability by adding new capabilities to grpc in the gobgp.proto
* add the ability to configure the new capability using the cli by adding addition commands to the gobgp management CLI
* add the ability to configure the new capability using the gobgpd configuration files in TOML, YAML, JSON and HCL


# Resources.
An MCP called gobgp allows access to the source tree to analyze the code base
An MCP called cobra allows access to the CLI source tree to analyze the code base
The go netlink library documentation is located at https://pkg.go.dev/github.com/vishvananda/netlink
The go-grpc source code is located in ../grpc-go



# Instructions.
1. Analyze the gobgp code base using the MCP called gobgp.  Take particular notice of the Zebra functionality which provides similar functionality redistributing routes to the Zebra routing system.  The functionality we are implementing is similar but simpler

2. Review the go netlink api documentation documentation.  

3. Review the cobra documentation, examples and code using the MCP called cobra.

4. Implement the netlink code necessary to import connected routes from the linux host.   This should be achieved by listing the interfaces in the linux host and getting addresses allocated to those interfaces and adding those addresses to the gobgp rib. If VRF is specified in the configuration, the routes should be imported into the specified vrf. The configuration should be structured  with a list of interfaces or regex indicating the interfaces to be imported.  

5.  Implement the logic to configure the import functionality via GRPC in gobgp.api

6.  Implement the logic to configure the gobgp configuration file, an example is below.

7.  Implement the CLI to configure route import functionality and display the status of the imported routes, an example is below 

8.  Implement the netlink code necessary to export goBGP RIB routes to the Linux host routing table.  The implementation should provide a mechanism to import all routes or only those tagged with a specific community.  By default the routes should be exported to the global RIB, if a VRF is configured routes should be exported from that VRF only.

9.  Implement the logic to configure the export functionality via GRPC in gobgp.api

10. Implement the logic to configure using the gobgp configuration file, an example is below

11.  Implement the CLI logic to configure route export and display the status of routes exported to the linux routing table, an example is below



# Configuration examples.

##  Redistribute routes using configuration file.  
The following configures gobgp to import routes.  The syntax of this example is TOML which is the default for gobgp, however gobpg supports yaml, json and hcl, they should be implemented.

Example for redistribute import
```toml
[global.config]

[redistribute]
   [redistribute.import]
    enabled = true
    vrf = vrf1
    [[interfaces]]
        [interfaces.config]
            interface = eth0
    [[interfaces]]
        [interfaces.config]
            interface = eth*

```
Example for redistribute export
```toml
[global.config]

[redistribute]
   [redistribute.export]
    enabled = true
    vrf = vrf1
    [[community]]
        [community.config]
            community-name = "cs0"
            community-list = ["100:100","200:200"]
            large-community-list = ["100:100:100", "200:200:200"]

```

## Configure and display routes using gobgp CLI
The follow example illustrates how to configure redistribution using the cli.

```
# enable redistribution import.  ifname can be a regex
% gobgp redistribution enable import <ifname> [ -vrf <vrf> ]

# enable redistribution export.  
% gobgp redistribution enable export [ <community-name> | <community-list> | <large-community-list>] [ -vrf <vrf> ]


# show redistribution
% gobgp redistribution show
Import redistribution: true
interfaces: eth0, eth*
vrf: vrf1
Export redistribution: true
vrf: vrf1
community-name: cs0
community-list: ["100:100","200:200"]
large-community-list: ["100:100:100", "200:200:200"]
```


