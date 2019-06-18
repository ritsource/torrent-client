package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"reflect"
	"strconv"

	"github.com/sirupsen/logrus"
)

// Peer represents a single peer (seeder), it's IP and Port
type Peer struct {
	IP   net.IP
	Port uint16
}

// "BitTorrent protocol"

// Download handles file peer protocol messaging and file download from the
// corresponding peer. First it creates a connection to the listening peer
// and writes message twith protocol specification ("BitTorrent protocol")
// and peer identifier to that, if the peer wants to share data it sends a
// simillar message back to our client. This is called a handshake. Once the
// handshake is successful we request files to be downloaded from the peer
// this is done via verious messages between clients, to learn more visit -
// https://wiki.theory.org/index.php/BitTorrentSpecification#Peer_wire_protocol_.28TCP.29
func (p *Peer) Download(torr *Torr) {
	// establishing TCP connection with the peer client
	conn, err := net.Dial("tcp", p.IP.String()+":"+strconv.Itoa(int(p.Port)))
	if err != nil {
		fmt.Printf("couldn't connect to peer %v - %v\n", conn.RemoteAddr(), err)
		return
	}
	defer conn.Close()

	// First we want to let the peer know what files you want and also
	// some unique identifier for our client, aka a Handshake message.

	// Building data to be sent as handshake message, to find more about it
	// https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake
	hsbuf, err := HandshakeBuf(torr) // buffer to be sent
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer - %v\n", err)
		return
	}

	// Writing the handshake data to the peer connection
	_, err = conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write teh handshake message to peer %v - %v\n:", conn.RemoteAddr(), err)
		return
	}
	// logrus.Infof("requested handshake to %v\n", conn.RemoteAddr())

	// Now, if the peer doesn’t have the files you want (info sent via-
	// info_hash in handshake request), they will close the connection,
	// but if they do then they should send back a similar message as
	// confirmation. We need to wait for the client to write back

	// http://allenkim67.github.io/programming/2016/05/04/how-to-make-your-own-bittorrent-client.html#downloading-from-peers
	// this article helped me a lot to understand peer to peer messeging

	// Simply, the client might not always send the data in 1 event, it
	// might send the messages in chunks as multiple events, we need to
	// take care of making sense of that data, and that's where the
	// message length data is useful at the start of the message

	// msgbuf will be used to store the message data while reading multiple times
	msgbuf := new(bytes.Buffer)
	// the first message is expected to be handshake type and the length defination
	// for handshake is different, so handshake is true at the start of reading, once
	// the first message has totally been read we will decleare the handshake as `false`
	handshake := true

	// Continiously reading from the peer client
	for {
		// Reading from the connection
		b := make([]byte, 1024)
		nr, err := conn.Read(b)
		if err != nil {
			if err != io.EOF {
				logrus.Warnf("%v\n", err)
				// logrus.Warnf("read error with %v -> %v\n", conn.RemoteAddr(), err)
			}
			continue // if error on read skip teh rest of this iretation
		}
		data := b[:nr] // useful part of `b`

		// Extracting message from the read data
		if nr > 0 {
			var explen int // expected length, indicates how far a messgae should be read a single message

			// for handshake message (generally the first message) length of message is different, so the
			// value of `explen` will be extracted differently, (49+len(x)); x = length of protocol-string
			// to learn more, https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake
			if handshake {
				// reading the protocol string ("BitTorrent protocol" for version 1) length
				bf := new(bytes.Buffer)
				bf.Write(data)
				l, err := bf.ReadByte() // l, length specified at the start of
				if err != nil {
					logrus.Warnf("error on handshake response - %v\n", err)
				}

				// explen = (49+len(x)); for handshake message
				explen = int(uint8(l)) + 49
			} else {
				// for all other message type, the first 4 bytes (uint32) defines
				// the length of message data. So, reading the data `explen` to that
				explen = int(binary.BigEndian.Uint32(b[:4])) + 4
			}

			// writing the crrently read data to `msgbuf` buffer (later will be reset once message read is complete)
			msgbuf.Write(data)

			// fmt.Println("explen -> ", explen)

			// once `msgbuf` lnegth is equal to (or greater than) `explen`,
			// the message read is complete, doing what to do _________
			if msgbuf.Len() >= 4 && msgbuf.Len() >= explen {
				// fmt.Printf("Okay %v %v %s\n", conn.RemoteAddr(), msgbuf.Len(), msgbuf.Bytes())

				if handshake && isHandshake(msgbuf.Bytes()) {
					logrus.Infof("handshake successful, %v, %v bytes\n", conn.RemoteAddr(), explen)
				} else {
					logrus.Infof("message recieved, %v bytes from %v\n", explen, conn.RemoteAddr())
				}

				msgbuf.Reset()    // reseting the `msgbuf` buffer, message read is complete
				handshake = false // handshake is settin g to false (can only be true for first message read)
			}
		}

	}

}

func isHandshake(b []byte) bool {
	if reflect.DeepEqual(b[1:20], []byte("BitTorrent protocol")) {
		return true
	}

	return false
}

func handleMsg(conn net.Conn, buf bytes.Buffer) {
	fmt.Printf("%s\n", buf.Bytes())
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
