package peer

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/ritwik310/torrent-client/info"
	"github.com/ritwik310/torrent-client/torrent"
	"github.com/sirupsen/logrus"
)

// PeerProtocolV1 - Peer protocol name, for bitorrent protocol version 1
var PeerProtocolV1 = []byte("BitTorrent protocol")

// Peer ..
type Peer struct {
	IP       net.IP
	Port     uint16
	Torrent  *torrent.Torrent
	Conn     net.Conn
	UnChoked bool
	Bitfield []int8
}

// Close .
func (p *Peer) Close() {
	if p.Conn != nil {
		p.Conn.Close()
	}
}

// Start .
func (p *Peer) Start() {
	// First we need to establish a rcp connection with the
	// remote peer p.Conn holds a pointer to teh connection
	var err error
	p.Conn, err = net.Dial("tcp", p.IP.String()+":"+strconv.Itoa(int(p.Port)))
	if err != nil {
		logrus.Warnf("%v\n", err)
		return
	}

	// now := time.Now()
	// conn.SetDeadline(now.Add(time.Second * 5))

	// Next, our client needs to send some unique identifier
	// of the torrent and our client, aka a handshake message

	// So, building the handshake message buffer
	hsbuf, err := handshakeBuf(p.Torrent)
	if err != nil {
		logrus.Warnf("couldn't build the handshake buffer, %v\n", err)
		return
	}

	// Writing the handshake message to the connection
	_, err = p.Conn.Write(hsbuf.Bytes())
	if err != nil {
		logrus.Warnf("couldn't write handshake request, %v\n", err)
		return
	}

	// Now, if the peer doesn’t have the files you want (info sent via-
	// info_hash in handshake request), they will close the connection,
	// but if they do then they should send back a similar message as
	// confirmation. We need to wait for the client to write back
	p.ReadMessages()
}

// handshakeBuf builds and returns a handshake message buffer
func handshakeBuf(torr *torrent.Torrent) (*bytes.Buffer, error) {
	// building the buffer, for the details -
	// https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint8(len(PeerProtocolV1))) // length of protocol name string, as a single raw byte
	err = binary.Write(buf, binary.BigEndian, PeerProtocolV1)              // string identifier of the protocol ("BitTorrent protocol")
	err = binary.Write(buf, binary.BigEndian, uint64(0))                   // eight (8) reserved bytes. All current implementations use all zeroes.
	err = binary.Write(buf, binary.BigEndian, torr.InfoHash)               // 20-byte string used as a unique ID for the client.
	err = binary.Write(buf, binary.BigEndian, info.ClientID)               // 20-byte string used as a unique ID for client, the same peer_id that is transmitted in tracker requests

	return buf, err
}

// isHandshake checks if a message data is handshake or not
func isHandshake(b []byte) bool {
	return len(b) >= 20 && reflect.DeepEqual(b[1:20], PeerProtocolV1)
}

// ReadMessages .
func (p *Peer) ReadMessages() {
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

	var explen int // expected length, indicates how far a messgae should be read a single message
	var start = true

	// continiously reading from the
	// client unless the connection fails
	for {
		time.Sleep(1 / 10 * time.Second)
		// reading from teh peer connection
		data := make([]byte, 1024)
		nr, err := p.Conn.Read(data)
		if err != nil {
			// if connection (err != io.EOF) is not
			// open anymore break the iretation
			// if err != io.EOF {
			logrus.Warnf("%v\n", err)
			break
			// }
			// // for other kind of errors, skip the rest of the iretation
			// continue
		}

		// if read data length is 0, i.e. no message
		// then skip teh rest of the message handling
		if nr == 0 {
			continue
		}

		// data recieved from the peer
		b := data[:nr]

		if nr > 0 {

			if start {

				// for handshake message (generally the first message) length of message is different, so the
				// value of `explen` will be extracted differently, (49+len(x)); x = length of protocol-string
				// to learn more, https://wiki.theory.org/index.php/BitTorrentSpecification#Handshake
				if handshake {
					// reading the protocol string ("BitTorrent protocol" for version 1) length
					bf := new(bytes.Buffer)
					bf.Write(b)
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
			}

			fmt.Println("explen **********", explen)

			start = false
			// writing the crrently read data to `msgbuf` buffer (later will be reset once message read is complete)
			msgbuf.Write(b)

			// fmt.Println("explen -> ", explen)

			// once `msgbuf` lnegth is equal to (or greater than) `explen`,
			// the message read is complete, doing what to do _________
			if msgbuf.Len() >= 4 && msgbuf.Len() >= explen {
				// fmt.Printf("Okay %v %v %s\n", conn.RemoteAddr(), msgbuf.Len(), msgbuf.Bytes())

				p.HandleMessages(msgbuf)

				start = true
				msgbuf.Reset()    // reseting the `msgbuf` buffer, message read is complete
				handshake = false // handshake is settin g to false (can only be true for first message read)
			}
		}
	}
}

// HandleMessages .
func (p *Peer) HandleMessages(buf *bytes.Buffer) {
	if isHandshake(buf.Bytes()) {
		logrus.Infof("handshake message recieved - %v\n", p.Conn.RemoteAddr())

		return
		// writing interested message
		buf := new(bytes.Buffer)
		binary.Write(buf, binary.BigEndian, uint32(1))
		binary.Write(buf, binary.BigEndian, uint8(2))

		p.Conn.Write(buf.Bytes())

		return
	}

	b := buf.Bytes()

	if len(b) < 4+1 {
		logrus.Errorf("invalid message - %v", p.Conn.RemoteAddr())
		return
	}

	length := binary.BigEndian.Uint32(b[:4])

	if len(b) < int(length)+4 || length == 0 {
		logrus.Errorf("invalid message - %v", p.Conn.RemoteAddr())
		return
	}

	fmt.Println("len ->", len(b))
	id := b[4]

	var payload []byte
	if len(b) > 5 {
		payload = b[5:]
	}

	logrus.Infof("message length %v bytes", len(b))
	fmt.Printf("🍎%v\n", b)

	switch id {
	case uint8(0):
		logrus.Infof("choke\n")
		p.Close()
	case uint8(1):
		logrus.Infof("unchoke\n")
		p.UnChoked = true
	case uint8(2):
		logrus.Infof("interested\n")
	case uint8(3):
		logrus.Infof("not interested\n")
	case uint8(4):
		logrus.Infof("have\n")
	case uint8(5):
		logrus.Infof("-> bitfield\n")
		p.handleBitfield(payload)
	case uint8(6):
		logrus.Infof("request\n")
	case uint8(7):
		logrus.Infof("piece\n")
	case uint8(8):
		logrus.Infof("cancel\n")
	case uint8(9):
		logrus.Infof("port\n")
	}

}

func (p *Peer) handleBitfield(payload []byte) {

	if len(p.Torrent.Pieces) != len(payload)*8 {
		logrus.Errorf("piece length mismatch in bitfield message")
		return
	}

	p.Bitfield = make([]int8, len(payload)*8)

	for i, b := range payload {
		for j := 0; j < 8; j++ {
			p.Bitfield[i*8+j] = int8(b >> uint(7-j) & 0x01)
		}
	}

}

// RequestPiece .
func (p *Peer) RequestPiece(b *torrent.Block) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.BigEndian, uint32(13))
	binary.Write(buf, binary.BigEndian, uint8(6))
	binary.Write(buf, binary.BigEndian, b.Index) // piece index
	binary.Write(buf, binary.BigEndian, b.Begin)
	binary.Write(buf, binary.BigEndian, b.Length)

	_, err := p.Conn.Write(buf.Bytes())
	if err != nil {
		// logrus.Errorf("couldn't write request message, %v\n", err)
		return
	}

	// logrus.Infof("written %v bytes as piece request to %v\n", nw, p.Conn.RemoteAddr())
	b.Status = torrent.BlockRequested

	// go func(bl *torrent.Block) {
	// 	time.Sleep(30 * time.Second)
	// 	if bl.Status != torrent.BlockDownloaded {
	// 		logrus.Errorf("ERRORRRRRR -> Block download failed %+v\n", bl)
	// 	}
	// }(b)
}
