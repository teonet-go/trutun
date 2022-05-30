#!/bin/sh
#
# Usual Linux post connect commands shell script.
# It set interface IP address and up the interseice.
#
TRU=$1 && sudo ip addr add $2 dev $TRU && sudo ip link set up dev $TRU
