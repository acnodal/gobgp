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

package table

import (
	"fmt"
	"net"
	"net/netip"

	"github.com/osrg/gobgp/v4/pkg/packet/bgp"
)

func NewNlriFromAPI(dst *net.IPNet) (bgp.PathNLRI, error) {
	if dst == nil {
		return bgp.PathNLRI{}, fmt.Errorf("nil dst")
	}
	addr, ok := netip.AddrFromSlice(dst.IP)
	if !ok {
		return bgp.PathNLRI{}, fmt.Errorf("invalid ip address: %s", dst.IP)
	}
	ones, _ := dst.Mask.Size()
	prefix := netip.PrefixFrom(addr.Unmap(), ones)

	nlri, err := bgp.NewIPAddrPrefix(prefix)
	if err != nil {
		return bgp.PathNLRI{}, err
	}
	return bgp.PathNLRI{NLRI: nlri}, nil
}
