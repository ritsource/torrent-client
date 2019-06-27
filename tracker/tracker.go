package tracker

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"

	"github.com/marksamman/bencode"
	"github.com/ritwik310/torrent-client/info"
	"github.com/ritwik310/torrent-client/peer"
	"github.com/ritwik310/torrent-client/torrent"
)

// NewTracker function returns a new Tracker struct
func NewTracker(torr *torrent.Torrent) Tracker {
	return Tracker{
		Torrent: torr,
		Peers:   []*peer.Peer{},
	}
}

// Tracker struct handles announce and tracker related methods
type Tracker struct {
	Torrent      *torrent.Torrent
	Peers        []*peer.Peer
	SharingPeers []*peer.Peer
}

// GetPeersHTTP sends tracker request to the announce address
// reads the relevent data (peers and stuff)
func (t *Tracker) GetPeersHTTP() (uint32, error) {
	// populating tracker announce url with
	// appropriate param values from the Torr
	trkurl, err := trackerurl(t.Torrent)
	if err != nil {
		return 0, err
	}

	// sending http tracker request to trkurl
	resp, err := http.Get(trkurl)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// check if everything has gone ok
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status-code not 200")
	}

	// The tracker responds with "text/plain" document consisting
	// of a bencoded dictionary. For more details about tracker response,
	// https://wiki.theory.org/index.php/BitTorrentSpecification#Tracker_Response

	// decoding response data (bencode)
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		panic(err)
	}

	// check if request faild ("failure reason" exist on the response)
	if v, ok := data["failure reason"]; ok {
		fmt.Println("Error: client is not authorized to download the torrent")
		return 0, fmt.Errorf("%v", v)
	}

	// Similar to failure reason, but the response still gets processed
	// normally, this warning message is shown just like an error
	if v, ok := data["warning message"]; ok {
		fmt.Printf("WARNING: %s\n", v)
	}

	// reading the minimum announce interval, if present clients must
	// not reannounce more frequently than this
	var interval uint32
	if v, ok := data["min interval"]; ok {
		interval = v.(uint32)
	}

	// the peers value might be a string consisting of multiples of 6 bytes,
	// first 4 bytes are the IP address and last 2 bytes are the port number,
	// all in network (big endian) notation.
	if str, ok := data["peers"].(string); ok {
		peers := []byte(str)
		i := 0
		for {
			if i >= len(peers) {
				// if end of string, break
				break
			}
			// reading peer info and appending it the the tracker struct,
			peer := peer.Peer{IP: net.IP(peers[i : i+4]), Port: binary.BigEndian.Uint16(peers[i+4 : i+6])}
			t.Peers = append(t.Peers, &peer) // appending peer to tracker.Peers
			// skip the next 6
			i += 6
		}
	}

	// TODO: the peer might also be a dictionary have to handle that case too

	// fmt.Printf("%+v\n", t.Peers)

	return interval, nil
}

// trackerurl returns announce string with all the required param values for tracker request
func trackerurl(torr *torrent.Torrent) (string, error) {
	// reading the url from torr data, torr["announce"]
	trkurl := torr.AnnounceURL

	// appending all the required param values for tracker request
	// to learn more about it https://wiki.theory.org/index.php/BitTorrentSpecification#Tracker_Request_Parameters
	pr := url.Values{}
	pr.Add("info_hash", string(torr.InfoHash))         // urlencoded 20-byte SHA1 hash of the info value in torr
	pr.Add("peer_id", string(info.ClientID))           // urlencoded 20-byte string used as a unique ID for the client, generated at startup
	pr.Add("port", strconv.Itoa(int(info.ClientPort))) // the port number that the client is listening on
	pr.Add("uploaded", "0")                            // total amount uploaded (0 at start)
	pr.Add("downloaded", "0")                          // total downloaded (0 at start)
	pr.Add("left", strconv.Itoa(torr.Size))            // left to download (full at start)
	pr.Add("compact", "1")                             // 1
	pr.Add("event", "started")                         // started
	pr.Add("ip", info.ClientIP.String())               // client's IP, (optional)
	// pr.Add("numwant", "200")                                    // client's IP, (optional)

	// the tracker announce url
	trkurl.RawQuery = pr.Encode()

	return trkurl.String(), nil
}

// ConnUDP sends connection request to announce address and returns
// the relevent response data (transaction_id, connection_id, and error)
func (t *Tracker) ConnUDP(addr string, tid uint32) (uint32, uint64, error) {
	// creating connection request packet, to be sent to the client. It includes,
	// for more details visit http://www.bittorrent.org/beps/bep_0015.html
	// 0-8 -> protocol_id -> 64-bit integer -> 0x41727101980 (magic constant)
	// 8-12 -> action -> 32-bit integer -> 0 (constant, 0 indicates a connection req)
	// 12-16 -> transaction_id -> 32-bit integer -> client identifier
	var el = []interface{}{uint64(0x41727101980), uint32(0), uint32(tid)} // temporarily holds the data in an array

	// writing the data to a buffer, to be send in the request
	buf := new(bytes.Buffer)
	for i, v := range el {
		// appending each element to the buffer
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			fmt.Println("buffer write failed for connection request build: i =", i)
			return 0, 0, err
		}
	}

	// UDP protocol doesn't esablish any connection between client and server, the
	// connection doesn't actually represents any actual connection in transition layer
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	// writing the data to the server
	nw, err := conn.Write(buf.Bytes())
	if err != nil {
		return 0, 0, err
	}
	fmt.Printf("written %v bytes to as udp connection request\n", nw)

	// reading the connection response recieved from the server,
	// it includes some data that requered for the announce request
	// 0-4 -> action -> 32-bit integer -> 0 (indicates a connection req)
	// 4-8 -> transaction_id -> 32-bit integer -> same as sent in the request
	// 8-16 -> connection_id -> 64-bit integer -> connection id
	resp := make([]byte, 16)
	nr, err := bufio.NewReader(conn).Read(resp)
	if err != nil {
		return 0, 0, err
	}
	fmt.Printf("read %v bytes from udp connection response\n", nr)

	// error check, otherwise len(resp) is less than 16 bytes,
	// it world fail to extract data from it
	if len(resp) < 16 {
		fmt.Printf("the response length is shorter then 16 bytes")
		return 0, 0, err
	}

	// returning as the actual types
	// TODO: returning as []byte, would be easier for resending the data (ex: connection_id)
	BE := binary.BigEndian
	return BE.Uint32(resp[4:8]), BE.Uint64(resp[8:16]), err
}

// GetPeersUDP sends a UDP announce request to the server, takes care
// of formatting request data and populating Tracker with peers and other relevent data
func (t *Tracker) GetPeersUDP(addr string, tid uint32, cid uint64) (uint32, error) {
	numseed := 20 // number of requested seeders

	// building buffer to be sent with the announce request
	buf, err := announceDataUDP(t.Torrent, tid, cid, numseed)
	if err != nil {
		return 0, err
	}

	// udp dial (IK, just because comments are sexy)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// writing to data to the udp server
	nw, err := conn.Write(buf.Bytes())
	if err != nil {
		return 0, err
	}
	fmt.Printf("written %v bytes to as udp announce request\n", nw)

	// reading tracker response
	resp := make([]byte, 20+numseed*6) // 20+numseed*6 is the largest possible value for response (cause numseed is finite)
	nr, err := bufio.NewReader(conn).Read(resp)
	if err != nil {
		return 0, err
	}

	fmt.Printf("read %v bytes from udp announce response\n", nr)
	resp = resp[:nr] // skipping rest of the bytes, only populated ones contains all the data

	// if len(resp) < 20, somethings wrong with the response
	if len(resp) < 20 {
		fmt.Printf("the response length is shorter than 20 bytes")
		return 0, err
	}

	// extracting data from response, the response is formatted like,
	// 0-4 -> 32-bit integer -> action -> 1 (announce), not needed now
	BE := binary.BigEndian

	// 4-8 -> 32-bit integer -> transaction_id
	transactionID := BE.Uint32(resp[4:8])
	if tid != transactionID {
		fmt.Printf("transaction_id did not match, %v != %v\n", tid, transactionID)
		return 0, err
	}

	interval := BE.Uint32(resp[8:12]) // 8-12 -> 32-bit integer -> interval (new announce req can not be made until interval seconds have passed)
	// leechers := BE.Uint32(resp[12:16]) // 12-16 -> 32-bit integer -> leechers
	seeders := BE.Uint32(resp[16:20]) // 16-20 -> 32-bit integer -> number of seeders

	fmt.Printf("announce response recieved, transaction_id = %v\n", transactionID)
	fmt.Printf("number of seeders found = %v *****\n", seeders)

	// 20-nr -> rest of the part contains peer (seeder) information, 6 bytes for each peer
	// first 4 bytes are IP address and last 2 bytes are port. Reading the peer info,
	// more about it http://www.bittorrent.org/beps/bep_0015.html
	i := 20 // for 21st byte
	for {
		if i >= len(resp) {
			// if end of resp data the break
			break
		}
		// reading peer info and appending it the the tracker struct,
		peer := peer.Peer{IP: net.IP(resp[i : i+4]), Port: binary.BigEndian.Uint16(resp[i+4 : i+6])}
		t.Peers = append(t.Peers, &peer)
		i += 6
	}

	fmt.Printf("%+v\n", t.Peers)

	return interval, nil
}

// announceDataUDP takes a *Torr and returns a formatted buffer
// that contains required elements for UDP announce requests
func announceDataUDP(torr *torrent.Torrent, tid uint32, cid uint64, numseed int) (*bytes.Buffer, error) {
	// constructing buffer for required for request packet,
	// for more details visit http://www.bittorrent.org/beps/bep_0015.html

	// to temporarily hold the data in an array
	var el = []interface{}{
		uint64(cid),                            // 0-8 -> connection_id -> connection_id recieved from connection response
		uint32(1),                              // 8-12 -> action -> 1, represents announce request
		uint32(tid),                            // 12-16 -> transaction_id -> transaction_id from conn-response
		torr.InfoHash,                          // 16-36 -> info_hash -> sha1 hash of encoded (bencode) info_hash property of torr metadata
		info.ClientID,                          // 36-56 -> peer_id -> used as a unique ID for the client, generated by the client at startup
		uint64(0),                              // 56-64 -> downloaded -> how much has been downloaded (0 at start)
		uint64(torr.Size),                      // 64-72 -> left -> how many bytes are yet to be downloaded
		uint64(0),                              // 72-80 -> uploaded -> how much has been uploaded
		uint32(2),                              // 80-84 -> event -> 2 (0: none; 1: completed; 2: started; 3: stopped)
		binary.BigEndian.Uint32(info.ClientIP), // 84-88 -> IP -> client's ip address
		uint32(0),                              // 88-92 -> key -> for identification (optional)
		uint32(numseed),                        // 92-96 -> num_want -> -1 is default (number of peers that the client would like to receive)
		info.ClientPort,                        // 96-98 -> port -> port that the client is listening on (typically 6881-6889
	}

	// writing the data to a buffer, to be send in the request
	buf := new(bytes.Buffer)
	for i, v := range el {
		// appending each element to the buffer
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			fmt.Println("buffer write failed for announce request build: i =", i)
			return buf, err
		}
	}

	return buf, nil
}
