package client

import (
	"log"
	"math/rand"
	"net"
	"strconv"
)

// GenPeerID generates a somewhat random peerid (not best solution)
func GenPeerID() []byte {
	return []byte("-TC0001-kwsSnkYwydys")
	// TODO: has to be constant for each connection

	var r string
	for i := 0; i < 12; i++ {
		r += strconv.Itoa(rand.Intn(9))
	}
	return []byte("-TC0001-" + r)[:20]
}

// GetClientIP returns local machines primary IP-address
func GetClientIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP
}
