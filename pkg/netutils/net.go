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

package net

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