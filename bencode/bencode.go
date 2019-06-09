package bencode

import (
	"os"

	"github.com/marksamman/bencode"
)

// ReadTorrent reads a `.torrent` file given it's path and returns an `interface{}`
func ReadTorrent(p string) (map[string]interface{}, error) {
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

	return torr, nil
}
