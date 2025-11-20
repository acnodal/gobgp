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

package netutils

import (
	"fmt"
	"net"

	"github.com/osrg/gobgp/v4/pkg/log"
)

type ConnectedRoute struct {
	Prefix  *net.IPNet
	NextHop net.IP
}

// GetGlobalUnicastRoutes returns a list of global unicast IP addresses
// for a given network interface.
func GetGlobalUnicastRoutes(interfaceName string, logger log.Logger) ([]*ConnectedRoute, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %w", interfaceName, err)
	}

	var routes []*ConnectedRoute
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ip := ipnet.IP
			isGlobal := !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsUnspecified()

			// Calculate the network address
			network := &net.IPNet{
				IP:   ipnet.IP.Mask(ipnet.Mask),
				Mask: ipnet.Mask,
			}

			logger.Debug("Found address on interface",
				log.Fields{
					"Topic":     "net",
					"Address":   ipnet.String(),
					"Network":   network.String(),
					"Interface": interfaceName,
					"IsGlobal":  isGlobal,
				})

			if isGlobal {
				routes = append(routes, &ConnectedRoute{
					Prefix:  network,
					NextHop: ip,
				})
			}
		}
	}

	routeStrings := make([]string, len(routes))
	for i, r := range routes {
		routeStrings[i] = r.Prefix.String()
	}

	logger.Debug("Returning routes from interface",
		log.Fields{
			"Topic":     "net",
			"Routes":    routeStrings,
			"Interface": interfaceName,
		})
	return routes, nil
}

// GetLinkLocalIPv6Address returns the link-local IPv6 address for a given
// network interface.
func GetLinkLocalIPv6Address(interfaceName string) (net.IP, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %w", interfaceName, err)
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ip := ipnet.IP
			if ip.To4() == nil && ip.IsLinkLocalUnicast() {
				return ip, nil
			}
		}
	}

	return nil, fmt.Errorf("no link-local IPv6 address found for interface %s", interfaceName)
}

// GetIPv4Nexthop returns the IPv4 address for a given network interface.
// This function looks for a global unicast IPv4 address on the interface.
func GetIPv4Nexthop(interfaceName string, logger log.Logger) (net.IP, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %w", interfaceName, err)
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ip := ipnet.IP
			// Check if it's IPv4 and is global unicast
			if ip.To4() != nil {
				isGlobal := !ip.IsLoopback() && !ip.IsLinkLocalUnicast() && !ip.IsMulticast() && !ip.IsUnspecified()
				if isGlobal {
					logger.Debug("Found IPv4 nexthop on interface",
						log.Fields{
							"Topic":     "net",
							"Interface": interfaceName,
							"Address":   ip.String(),
						})
					return ip, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no IPv4 address found for interface %s", interfaceName)
}

// GetInterfaceByIP returns the name of the network interface that has the given IP address.
func GetInterfaceByIP(ip net.IP) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("failed to list interfaces: %w", err)
	}

	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.Equal(ip) {
					return iface.Name, nil
				}
			}
		}
	}

	return "", fmt.Errorf("no interface found for IP %s", ip.String())
}

type IPv6Nexthops struct {
	Global    net.IP
	LinkLocal net.IP
}

func GetIPv6Nexthops(interfaceName string, logger log.Logger) (*IPv6Nexthops, error) {
	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find interface %s: %w", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil, fmt.Errorf("failed to get addresses for interface %s: %w", interfaceName, err)
	}

	nexthops := &IPv6Nexthops{}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok {
			ip := ipnet.IP
			if ip.To4() == nil {
				if ip.IsGlobalUnicast() {
					if nexthops.Global == nil {
						nexthops.Global = ip
					}
				} else if ip.IsLinkLocalUnicast() {
					if nexthops.LinkLocal == nil {
						nexthops.LinkLocal = ip
					}
				}
			}
		}
	}

	logger.Debug("Found IPv6 nexthops on interface",
		log.Fields{
			"Topic":     "net",
			"Interface": interfaceName,
			"Global":    nexthops.Global,
			"LinkLocal": nexthops.LinkLocal,
		})

	return nexthops, nil
}
