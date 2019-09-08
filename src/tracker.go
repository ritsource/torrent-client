package src

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
	"github.com/ritwik310/torrent-client/output"
)

// RequestPeerNum ...
var RequestPeerNum = 40

// GetPeers returns the peers
func GetPeers() ([]*Peer, error) {
	// check protocol
	switch Torr.Announce.Scheme {
	case "udp":
		// sending connection request to UDP server (the announce host) and reading responses
		connID, tranID, err := ConnReqUDP()
		if err != nil {
			return []*Peer{}, err
		}

		// once connection request is successfule, sending announce request
		// this will mainly get us a list of seeders for that torrent files
		return GetPeersUDP(Torr.Announce.String(), connID, tranID)

	case "http", "https":
		// if the announce scheme is http then send a http tracker request
		return GetPeersHTTP()

	default:
		return []*Peer{}, fmt.Errorf("unsupported announce protocol, %v", Torr.Announce.Scheme)
	}
}

/*
ConnReqUDP sends a UDP-connection-request to the tracker and returns
the relevent response data (connection_id and error)
*/
func ConnReqUDP() (uint64, uint32, error) {
	// building the required packet in connection request
	packet, err := udpConnPacket(TransactionID)
	if err != nil {
		return 0, 0, err
	}

	// UDP protocol doesn't esablish any connection between client and server, the
	// connection doesn't actually represents any actual connection in transition layer
	conn, err := net.Dial("udp", Torr.Announce.String())
	if err != nil {
		return 0, 0, err
	}
	defer conn.Close()

	// writing the data to the server (tracker-server)
	nw, err := conn.Write(packet)
	if err != nil {
		return 0, 0, err
	}

	output.DevInfof("written %v bytes as udp connection request\n", nw)

	// reading on the connection (waiting for teh server to respond in
	// actual) for a UDP-connection-response recieved from the server
	resp := make([]byte, 16)
	nr, err := bufio.NewReader(conn).Read(resp)
	if err != nil {
		return 0, 0, err
	}

	output.DevInfof("read %v bytes as udp connection response\n", nr)

	// the connection response includes data that is required in the announce request
	// `action` (32-bit integer, value = 0), `transaction_id` (32-bit integer, same value
	// as in the request), and `connection_id` (64-bit integer, required in announce request)
	// [0-4] -> action, [4-8] -> transaction_id, [8-16] -> connection_id

	// if response length is less than 16 bytes, it's not invalid
	if len(resp) < 16 {
		return 0, 0, fmt.Errorf("the response length is shorter then 16 bytes")
	}

	// extracting the information from response
	BE := binary.BigEndian

	// checking if the response (recieved from tracker), has the same `transition_id`
	// that was in the request, if not then it's not a successful transaction
	tID := BE.Uint32(resp[4:8])
	if tID != TransactionID {
		return 0, 0, fmt.Errorf("the response doesn't have the expected `transaction_id`")
	}

	// returning the `connection_id` which is going
	// to be required in UDP-accounce-request
	return BE.Uint64(resp[8:16]), tID, err
}

/*
GetPeersUDP sends a UDP announce request to the tracker and extracts
information about the peers from the response sent back by the tracker
*/
func GetPeersUDP(addr string, connID uint64, tranID uint32) ([]*Peer, error) {
	// building the required packet in connection request
	packet, err := udpAnnouncePacket(connID, tranID)
	if err != nil {
		return []*Peer{}, err
	}

	// udp dial (WTF, just because comments are cool)
	conn, err := net.Dial("udp", addr)
	if err != nil {
		return []*Peer{}, err
	}
	defer conn.Close()

	// writing to data to the udp server
	nw, err := conn.Write(packet)
	if err != nil {
		return []*Peer{}, err
	}
	output.DevInfof("written %v bytes to as udp announce request\n", nw)

	// reading tracker response
	resp := make([]byte, 20+RequestPeerNum*6) // 20 + `RequestPeerNum` * 6 is the largest possible value for response (cause `RequestPeerNum` is finite)
	nr, err := bufio.NewReader(conn).Read(resp)
	if err != nil {
		return []*Peer{}, err
	}

	output.DevInfof("read %v bytes as udp announce response\n", nr)
	resp = resp[:nr] // skipping rest of the bytes, only populated ones contains all the data

	// if len(resp) < 20, somethings wrong with the response
	if len(resp) < 20 {
		output.DevInfof("the announce response length is shorter than 20 bytes")
		return []*Peer{}, err
	}

	// response packet structure
	// 0-4 -> 32-bit integer -> action -> 1 (announce), not needed now
	// 4-8 -> 32-bit integer -> transaction_id
	// 8-12 -> 32-bit integer -> interval (new announce req can not be made until interval seconds have passed)
	// 12-16 -> 32-bit integer -> leechers
	// 16-20 -> 32-bit integer -> number of seeders
	// [rest] -> information about other peers

	// to extract data from response, the response is formatted like,
	BE := binary.BigEndian

	// checking if the response (recieved from tracker), has the same `transition_id`
	// that was in the request, if not then it's not a successful transaction
	if BE.Uint32(resp[4:8]) != TransactionID {
		return []*Peer{}, fmt.Errorf("the response doesn't have the expected `transaction_id`")
	}

	// interval := BE.Uint32(resp[8:12])
	// leechers := BE.Uint32(resp[12:16])
	// numseed := BE.Uint32(resp[16:20])
	// output.DevInfof("number of seeders found = %v *****\n", numseed)

	// [After 20-bytes] - the rest contains peer (seeder) information, 6 bytes for each peer
	// first 4 bytes are IP address and last 2 bytes are port. Reading the peer info,
	// more about it http://www.bittorrent.org/beps/bep_0015.html

	// holds all the peer pointers
	peers := []*Peer{}

	i := 20 // i = 21st byte

	// reading the data after 20-bytes, and extracting information about other peers
	// maximux needed peers <= 12
	for {
		if i >= len(resp) || i >= RequestPeerNum*6 {
			break
		}

		peers = append(peers, &Peer{IP: net.IP(resp[i : i+4]), Port: binary.BigEndian.Uint16(resp[i+4 : i+6])})

		i += 6
	}

	// returning the extracted information about peers
	return peers, err
}

/*
udpConnPacket builds and returns data required to be sent
in the packet for UDP-connection-request to the tracker

Packet structure looks like,
[0-8] -> `protocol_id` -> a `protocol_id` (0x41727101980, 64-bit integer, magic constant)
[8-12] -> `action` -> corrosponds to the action, value = 0 indicates a connection request (32-bit integer)
[12-16] -> `transaction_id` -> a `transaction_id`, client identifier (32-bit integer)
*/
func udpConnPacket(tranID uint32) ([]byte, error) {
	// el holds the contents of the packet
	var el = []interface{}{uint64(0x41727101980), uint32(0), uint32(tranID)}

	// building the connection request packet buffer
	buf := new(bytes.Buffer)

	// writing the data into the buffer
	for _, v := range el {
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			output.DevErrorf("buffer write failed for UDP connection request\n")
			return []byte{}, err
		}
	}

	return buf.Bytes(), nil
}

/*
udpAnnouncePacket builds and returns data required to be
sent in the packet for UDP-announce-request to the tracker

Packet structure looks like,
[0-8] -> `connection_id` -> `connection_id` recieved from UDP-connection-response (64-Bit integer)
[8-12] -> `action` ->  1, represents announce request (32-Bit integer)
[12-16] -> `transaction_id` ->  `transaction_id` from UDP-connection-response (32-Bit integer)
[16-36] -> `info_hash` -> sha1 hash of encoded (bencode) info_hash property of torr metadata (20-bytes long)
[36-56] -> `peer_id` -> used as a unique ID for the client, generated by the client at startup (20-bytes long)
[56-64] -> `downloaded` -> how much has been downloaded (value will be 0, at start) (64-Bit integer)
[64-72] -> `left` -> how many bytes are yet to be downloaded (64-Bit integer)
[72-80] -> `uploaded` -> how much has been uploaded (64-Bit integer)
[80-84] -> `event` -> 2 (0: none; 1: completed; 2: started; 3: stopped) (32-Bit integer)
[84-88] -> `IP` -> client's ip address (32-Bit integer)
[88-92] -> `key` -> for identification (optional) (32-Bit integer)
[92-96] -> `num_want` -> -1 is default (number of peers that the client would like to receive) (32-Bit integer)
[96-98] -> `port` -> port that the client is listening on (typically 6881-6889 (32-Bit integer)
*/
func udpAnnouncePacket(connID uint64, tranID uint32) ([]byte, error) {
	// `el` temporarily holds the data in an array
	var el = []interface{}{
		connID,
		uint32(1),
		tranID,
		Torr.InfoHash,
		PeerID,
		uint64(0),
		uint64(Torr.Size),
		uint64(0),
		uint32(2),
		ClientIP,
		uint32(0),
		uint32(RequestPeerNum),
		ClientPort,
	}

	// writing the data to a buffer, to be send in the request
	buf := new(bytes.Buffer)
	for _, v := range el {
		// appending each element to the buffer
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			output.DevErrorf("buffer write failed for UDP announce request\n")
			return []byte{}, err
		}
	}

	return buf.Bytes(), nil
}

/*
GetPeersHTTP sends a HTTP announce request to the tracker
and gets information about other peers
*/
func GetPeersHTTP() ([]*Peer, error) {
	// trkurl is the address for sending the announce request
	trkurl := Torr.Announce

	// to populate the URL query values with required properties,
	// torrent identifier, client information, the data we want
	// to download and number of peers we want
	pr := url.Values{}

	pr.Add("info_hash", string(Torr.InfoHash))      // torrent info_hash, sha1 hash of encoded (bencode) info_hash property of torr metadata
	pr.Add("peer_id", PeerID)                       // peer_id, unique identifier for each download
	pr.Add("port", strconv.Itoa(int(ClientPort)))   // post that out client is listening on for sharing data
	pr.Add("ip", ClientIP.String())                 // ip address of the local machine
	pr.Add("uploaded", "0")                         // how much data has been uploaded
	pr.Add("downloaded", "0")                       // how much data has been downloaded
	pr.Add("left", strconv.Itoa(Torr.Size))         // how much data is left to be downloaded
	pr.Add("compact", "1")                          // 1
	pr.Add("event", "started")                      // what event this announce request is for
	pr.Add("numwant", strconv.Itoa(RequestPeerNum)) //  number of peers we want the server to send back

	trkurl.RawQuery = pr.Encode()

	// sending the announce request to `trkurl`
	resp, err := http.Get(trkurl.String())
	if err != nil {
		return []*Peer{}, err
	}
	defer resp.Body.Close()

	// checking if request failed
	if resp.StatusCode != http.StatusOK {
		return []*Peer{}, fmt.Errorf("tracker request failed: %v", resp.StatusCode)
	}

	// decoding the data got back from in the response
	data, err := bencode.Decode(resp.Body)
	if err != nil {
		output.DevErrorf("unable to decode the tracker response data, %v", err)
		return []*Peer{}, err
	}

	// checking if tracker rejected the request
	if v, ok := data["failure reason"]; ok {
		return []*Peer{}, fmt.Errorf("Tracker request rejected: %v", v)
	}

	// if there's any warnning
	if v, ok := data["warning message"]; ok {
		output.DevWarnf("%v\n", v)
	}

	// to hold pointers to peers
	peers := []*Peer{}

	// reading information about peers
	if str, ok := data["peers"].(string); ok {
		d := []byte(str)

		i := 0
		for {
			// max number of peers 12
			if i >= len(d) || i >= RequestPeerNum*6 {
				break
			}

			p := Peer{
				IP:   net.IP(d[i : i+4]),
				Port: binary.BigEndian.Uint16(d[i+4 : i+6]),
			}

			peers = append(peers, &p)

			i += 6
		}
	}

	return peers, err
}
