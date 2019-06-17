package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
)

// Peer represents a single peer (seeder), it's IP and Port
type Peer struct {
	IP   net.IP
	Port uint16
}

// "BitTorrent protocol"

// HandleMessaging ...
func (p *Peer) HandleMessaging(torr *Torr) {
	conn, err := net.Dial("tcp", p.IP.String()+":"+strconv.Itoa(int(p.Port)))
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	defer conn.Close()

	hsbuf, err := HandshakeBuf(torr)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	_, err = conn.Write(hsbuf.Bytes())
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Printf("Handshaking with %v\n", conn.RemoteAddr())

	// readMsg(conn)

	for {
		buf := new(bytes.Buffer)

		// the length data is saved on a (uint8 at the start of message),
		// that means max size of a message is not going to be more than 255+1 (for handshake 255+49)
		b := make([]byte, 512)
		nr, err := conn.Read(b)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("Error:", err)
		}

		// fmt.Printf("%s\n", b)

		if nr > 0 {
			buf.Write(b[:nr])
			fmt.Printf("Read %v bytes from %v\n", nr, conn.RemoteAddr())
			handleMsg(conn, *buf)
		}

	}

	// fmt.Println("Finished!")
}

func handleMsg(conn net.Conn, buf bytes.Buffer) {
	// fmt.Printf("%s\n", buf.Bytes())
	b := buf.Bytes()
	msglen, err := buf.ReadByte()
	if err != nil {
		fmt.Println("couldn't read message length (first byte)", err)
		return
	}

	if len(b) == int(msglen)+49 {
		fmt.Printf("Handshake successful with %v\n", conn.RemoteAddr())
	}
}

func (p *Peer) onChoke(conn net.Conn) {
	fmt.Printf("OnChoke - closing connection with %v\n", conn.RemoteAddr())
	conn.Close()
}

// HandshakeBuf builds and returns data to be sent on a handshake
// request to the peer, what is required message and must be the
// first message transmitted by the client after the TCP connection
// is established to each peer client
func HandshakeBuf(torr *Torr) (*bytes.Buffer, error) {
	// building buffer to be sent for handshake, for more details
	// https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake
	// simply looks like, pstrlen + pstr + reserved + info_hash + peer_id
	// for version 1.0 of the BitTorrent protocol, pstrlen = 19, and pstr = "BitTorrent protocol".
	var el = []interface{}{
		uint8(19),                      // pstrlen -> string length of <pstr>, as a single raw byte
		[]byte("BitTorrent protocol"),  // pstr -> string identifier of the protocol ("BitTorrent protocol")
		uint64(0),                      // reserved -> eight (8) reserved bytes. All current implementations use all zeroes.
		infohash((*torr).Data["info"]), // peer_id -> 20-byte string used as a unique ID for the client.
		genpeerid(),                    // 20-byte string used as a unique ID for client, the same peer_id that is transmitted in tracker requests
	} // temporarily holds the data in an array

	// writing the data to a buffer, formatted for handshake
	buf := new(bytes.Buffer)
	for i, v := range el {
		// appending each element to the buffer
		err := binary.Write(buf, binary.BigEndian, v)
		if err != nil {
			fmt.Println("buffer write failed for handshake build: i =", i)
			return buf, err
		}
	}

	return buf, nil
}

// the following methods are responsible for sending various event
// messages over the connection to the peer client, for more details
// https://wiki.theory.org/index.php/BitTorrentSpecification#Messages
// also, https://www.bittorrent.org/beps/bep_0003.html

// the message data simply looks something like <length prefix> <message ID> <payload> (defined by the protocol)
// length prefix -> is a uint32 that indicates the total length of the <message ID> and <payload>
// message ID -> defines the type is the message, an uint8
// payload -> is the payload requested if there's any (usually downloadable data)

// KeepAlive sends a keep-alive message to the peer-client it
// must be sent to maintain the connection alive if no command
// have been sent for a given amount of time (generally two minutes)
func (p Peer) KeepAlive(c net.Conn) error {
	// for keep-alive message there's (no id, no payload),
	// it's just 4 bytes of containing 0
	return writetoconn(c, make([]byte, 4))
}

// Choke message sender
func (p Peer) Choke(c net.Conn) error {
	// The choke message has an id of 0 and has no payload
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1)) // fixed-length just the id prop
	binary.Write(buf, binary.BigEndian, uint8(0))  // id => 0 (for choke message)

	return writetoconn(c, buf.Bytes())
}

// UnChoke message sender
func (p Peer) UnChoke(c net.Conn) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint8(1))

	return writetoconn(c, buf.Bytes())
}

// Interested message sender
func (p Peer) Interested(c net.Conn) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint8(2))

	return writetoconn(c, buf.Bytes())
}

// NotInterested message sender
func (p Peer) NotInterested(c net.Conn) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(1))
	binary.Write(buf, binary.BigEndian, uint8(3))

	return writetoconn(c, buf.Bytes())
}

// Have message sender
func (p Peer) Have(c net.Conn, pi uint32) error {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(5))
	binary.Write(buf, binary.BigEndian, uint8(4))
	binary.Write(buf, binary.BigEndian, pi)

	return writetoconn(c, buf.Bytes())
}

// writetoconn writes data to a connection and returns
// error if there's an error
func writetoconn(c net.Conn, b []byte) error {
	_, err := c.Write(b)
	if err != nil {
		return err
	}
	return nil
}
