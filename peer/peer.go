package peer

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"reflect"
	"strconv"

	"github.com/ritwik310/torrent-client/client"
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
func (p *Peer) Download(torr *client.Torr) {
	// establishing TCP connection with the peer client
	conn, err := net.Dial("tcp", p.IP.String()+":"+strconv.Itoa(int(p.Port)))
	if err != nil {
		logrus.Warnf("%v\n", err)
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
					logrus.Infof("handshake successful, %v bytes from %v\n", explen, conn.RemoteAddr())
				} else {
					logrus.Infof("message recieved, %v bytes from %v\n", explen, conn.RemoteAddr())
					handleMessages(conn, msgbuf)
				}

				msgbuf.Reset()    // reseting the `msgbuf` buffer, message read is complete
				handshake = false // handshake is settin g to false (can only be true for first message read)
			}
		}

	}

}

// isHandshake checks if a message data is handshake or not
func isHandshake(b []byte) bool {
	return reflect.DeepEqual(b[1:20], []byte("BitTorrent protocol"))
}

func handleMessages(conn net.Conn, buf *bytes.Buffer) {
	if isHandshake(buf.Bytes()) {
		writeInterested(conn)
	} else {
		var len uint32
		var id uint8
		binary.Read(buf, binary.BigEndian, &len)
		binary.Read(buf, binary.BigEndian, &id)

		payload := make([]byte, len-1)
		binary.Read(buf, binary.BigEndian, &payload)

		switch id {
		case uint8(0):
			logrus.Info("choke")
			err := handleChoke(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(1):
			logrus.Info("unchoke")
			err := handleUnchoke(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(2):
			logrus.Info("interested")
			err := handleInterested(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(3):
			logrus.Info("not interested")
			err := handleNotInterested(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(4):
			logrus.Info("have")
			err := handleHave(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(5):
			logrus.Info("bitfield")
			err := handleBitfield(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(6):
			logrus.Info("request")
			err := handleRequest(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(7):
			logrus.Info("piece")
			err := handlePiece(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(8):
			logrus.Info("cancel")
			err := handlePort(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		case uint8(9):
			logrus.Info("port")
			err := handleChoke(conn)
			if err != nil {
				logrus.Errorf("%v\n", err)
			}
		}
	}
	// fmt.Printf("%s\n", buf.Bytes())
}
