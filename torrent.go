package main

import (
	"crypto/sha1"
	"fmt"
	"net/url"
	"os"

	"github.com/marksamman/bencode"
)

// NewTorrent gets a file path and constructs a Torrent struct from that
func NewTorrent(fn string) (*Torrent, error) {
	t := Torrent{}

	// reading data from the torrent file
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// decoding torrent file data (bencode)
	bd, err := bencode.Decode(f)
	if err != nil {
		return nil, err
	}

	// reading the announce url from bencode data
	t.AnnounceURL, err = url.Parse(bd["announce"].(string))
	if err != nil {
		return nil, err
	}

	// TODO: handling single file download for now

	// calculating infohash, every torrent is uniquely identified by
	// its infohash, required while sending handshake request and more
	// it's a 20 byte long sha1 hash of bencode encoded info value
	enc := bencode.Encode(bd["info"])
	h := sha1.New()
	h.Write(enc)
	t.InfoHash = h.Sum(nil)

	// reading the other required peoperties from the info value
	// pieces, size and infohash of the torrent
	if info, ok := bd["info"].(map[string]interface{}); ok {
		pieces := []byte(info["pieces"].(string)) // the concatenation of all 20-byte SHA1 hash values
		pl := int(info["piece length"].(int64))   // length of each piece (the file to be downloaded) in bytes, it's equal for every piece

		// reading pieces and appending each hash to t.Pieces property
		for i := 0; i+20 <= len(pieces); i += 20 {
			t.Pieces = append(t.Pieces, Piece{Status: 0, Hash: pieces[i : i+20]})
		}

		t.PieceLen = pl
		t.Size = pl * len(t.Pieces) // total size of downloadable file

		// fmt.Println("--__--__--__-->", len(pieces)/20, len(t.Pieces))
	} else {
		return nil, fmt.Errorf("torrent file read: unable to read torrent info")
	}

	return &t, nil
}

// Torrent struct represents the meta-data
// of a selected torrent file for downloading
type Torrent struct {
	Data        map[string]interface{} // bencode decoded metainfo
	Pieces      []Piece                // all the pieces inf the info property
	Size        int                    // total size of teh file to be downloaded in bytes
	PieceLen    int                    // piece length property (length of each piece in bytes)
	InfoHash    []byte                 // infohash of the torrent file
	AnnounceURL *url.URL               // announce URL
}

// constants required for representing the state of a peer
const (
	PieceNotFound   uint8 = 0 // Initial state, and when no bitfield or have request contains the piece-index
	PieceFound      uint8 = 1 // when atleast 1 have/bitfield request contains teh piece-index
	PieceRequested  uint8 = 2 // when piece whave been requested to a peer
	PieceDownloaded uint8 = 3 // when piece download has successfully been completed
)

// Piece represents a chunk of the actual file that needed to be downloaded
// from peers, idealy different pieces needs to be downloaded from different
// peers to make the download efficient
type Piece struct {
	Status uint8
	Hash   []byte
	Length int
	// Peers - peers who have that (probably)
}
