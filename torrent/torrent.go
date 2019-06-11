package torrent

import (
	"crypto/sha1"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"

	"github.com/marksamman/bencode"
)

// readfile reads a `.torrent` file, decodes bencode dictionary
// and returns a map[string]interface{} with contains torrnt metainfo
func readfile(p string) (*map[string]interface{}, error) {
	// reading a torrent file
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	// decoding torrent file data
	torr, err := bencode.Decode(f)
	if err != nil {
		return nil, err
	}

	return &torr, nil
}

// trackerparams returns all the required param values for tracker request
func trackerparams(torr *map[string]interface{}) map[string]string {
	var totalbytes int // total number of bytes calculated from torr["info"] values

	// calculating totalbytes, total length of all files
	if info, ok := (*torr)["info"].(map[string]interface{}); ok {
		var pl int // pl holds the value of `piece length`, length of each piece in bytes (its equal for all pieces)

		// iretating over info and reading necessary fields
		for k, v := range info {
			switch k {
			case "piece length":
				pl = int(v.(int64)) // each piece length
			case "pieces":
				// `pieces` contains of hashed values for all files, each hash is 20 bytes long,
				// so deviding the length of `pieces's value` will give us the number of pieces,
				// and multiplying it with `piece length` will be the total size of all pieces
				totalbytes = pl * (len([]byte(v.(string))) / 20)
			}
		}
	}

	// bullding and returning a map that contains all the required param values for tracker request
	return map[string]string{
		"info_hash":  string(infohash((*torr)["info"])),
		"peer_id":    genpeerid(),
		"port":       "6888",
		"uploaded":   "0",
		"downloaded": "0",
		"left":       strconv.Itoa(totalbytes),
		"compact":    "1",
		"event":      "started",
		"ip":         getclientip().String(),
	}
}

// getclientip returns local machines primary IP-address
func getclientip() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP
}

// infohash gets value of info from metainfo map, and
// returns a 20 byte long sha1 hash of all the info contents
func infohash(info interface{}) []byte {
	enc := bencode.Encode(info)
	h := sha1.New()
	h.Write(enc)
	hash := h.Sum(nil)

	return hash
}

// genpeerid generates a somewhat random peerid (not best solution)
func genpeerid() string {
	var r string
	for i := 0; i < 12; i++ {
		r += strconv.Itoa(rand.Intn(9))
	}
	return "-TC0001-" + r
}
