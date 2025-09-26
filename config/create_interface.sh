#!/bin/bash

# This script creates a temporary network interface configuration file for a specified interface.

# Usage: ./create_interface.sh <interface_name> <ip_address> <netmask>


if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <interface_name> <ip_address/mask>"
    exit 1
fi

interface_name=$1
ip_address=$2

echo "Creating configuration for interface $interface_name with IP $ip_address"   

ip link add $interface_name type dummy
ip addr add $ip_address dev $interface_name
ip link set $interface_name up

ip addr show   $interface_name