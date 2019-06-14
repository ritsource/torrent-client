package src

import (
	"os"

	"github.com/marksamman/bencode"
)

// Torr represents mets data from the torrent file
type Torr map[string]interface{}

// ReadFile ...
func (t *Torr) ReadFile(fp string) error {
	// reading a torrent file
	f, err := os.Open(fp)
	if err != nil {
		return err
	}

	// decoding torrent file data
	bd, err := bencode.Decode(f)
	if err != nil {
		return err
	}

	// torr := Torr(bd)
	*t = Torr(bd)

	return nil
}
