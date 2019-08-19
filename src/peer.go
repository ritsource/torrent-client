package src

import "net"

// PeerProtocolNameV1 .
var PeerProtocolNameV1 = []byte("BitTorrent protocol")

// Peers .
var Peers = []*Peer{}

// Peer .
type Peer struct {
	IP   net.IP
	Port uint16
}
