package src

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/ritwik310/torrent-client/output"
)

// Something ...
var (
	PeerID        string
	ClientIP      net.IP
	ClientPort    uint32
	TransactionID uint32
)

func init() {
	var err error

	// random seed
	rand.Seed(time.Now().UnixNano())

	// new peer id, to be generated once for every download
	PeerID = GenPeerID()

	// the client's IP-address
	ClientIP, err = GetClientIP()
	if err != nil {
		panic(fmt.Errorf("%v", err))
	}

	// in this application, we are not gonna focused
	// handling requests from other clients. So, just
	// a static port to be used in the tracker-request
	ClientPort = 6881

	// fmt.Printf("%s\n", PeerID)

	TransactionID = uint32(time.Now().Unix())
}

// GenPeerID generates a psudorandom peer-ID
func GenPeerID() string {
	// `[]rune` holding all the characters
	rsl := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	// generating a random sequence of character runes
	b := make([]rune, 12)
	for i := range b {
		b[i] = rsl[rand.Intn(len(rsl))]
	}

	// "-TC0001-" is client's unique id and version information
	return "-TC0001-" + string(b)
}

// GetClientIP retrives the host machine's IP-address
func GetClientIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		output.DevErrorf("couldn't read client's IP-address, %v\n", err)
		return nil, err
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP, nil
}
