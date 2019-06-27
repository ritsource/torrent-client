package info

import (
	"math/rand"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

var (
	// ClientID stores the PeerID for the client (new each time)
	ClientID []byte
	// ClientIP stores the IP address of the client
	ClientIP net.IP
	// ClientPort is the port number that client listens to
	ClientPort uint32
	// TransactionID a random ID, required for announce
	TransactionID uint32
)

func init() {
	// Generating a somewhat random ClientID (PeerID)
	rand.Seed(time.Now().UnixNano())
	letters := []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	id := []byte("-TC0001-")

	// Generating the random part of PeerIP, to learn more -
	// https://wiki.theory.org/index.php/BitTorrentSpecification#peer_id
	b := make([]byte, 12)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	id = append(id, b...)
	ClientID = id // ClientID

	// Reading the client's IP-address
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logrus.Fatalf("couldn't read client's IP-address, %v\n", err)
		return
	}
	defer conn.Close()

	ClientIP = conn.LocalAddr().(*net.UDPAddr).IP // ClientIP

	TransactionID = uint32(time.Now().Unix()) // WARNING: not sure if a good idea
}
