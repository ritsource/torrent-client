package src

import (
	"crypto/sha1"
	"fmt"
	"net/url"
	"os"
	"path"

	"github.com/marksamman/bencode"
	"github.com/sirupsen/logrus"
)

// Torr holds teh values read from
// the provided `.torrent` file
var Torr *Torrent

func init() {
	// reading the `.torrent` file from the command-line arguements
	if len(os.Args) < 2 {
		logrus.Panicf("no `.torrent` file provided")
	}
	fn := os.Args[1]

	// reading the `.torrent` file
	f, err := os.Open(fn)
	if err != nil {
		logrus.Panicf("no `.torrent` file provided")
	}
	defer f.Close()

	// decoding bencode dictionary into map[string]interface{}
	dict, err := bencode.Decode(f)
	if err != nil {
		logrus.Errorf("unable to decode `.torrent` file")
		logrus.Panicf("err")
	}

	Torr = &Torrent{}

	// populating properties of `Torr`
	// from the decoded dictionary
	Torr.Read(&dict)
}

// File holds value for every file to be downloaded
type File struct {
	Path   string // the path where the file needs to be written
	Length uint32 // size of the file (in bytes)
}

// Piece represents an individual piece of data
type Piece struct {
	Index uint32 // piece-index
	Hash  []byte // 20-byte long SHA1-hash of the piece-data, extracted from `.torrent` file
}

// Constants corrosponding to status enum value of of `Block`
const (
	BlockExist      uint8 = 0 // when piece whave been requested to a peer
	BlockRequested  uint8 = 1 // when piece whave been requested to a peer
	BlockDownloaded uint8 = 2 // when piece download has successfully been completed
	BlockFailed     uint8 = 3 // when piece download has not been successful (failed once)
)

// Block represents a block of data (a chunk of piece)
type Block struct {
	PieceIndex uint32 // piece-index of the piece that the block is a part of
	Begin      uint32 // offset where the block starts within the piece (that it's a part of)
	Length     uint32 // length of the block in bytes
	Status     uint8  // status of the block - exist (default), requested, downloaded, failed
}

// Constants corrosponding to file-mode enum value of `Torrent`
const (
	TorrSingleFile uint8 = 0 // Represents single file torrents
	TorrMultiFile  uint8 = 1 // Represents multi file torrents
)

// Torrent holds necesary data aquired from `.torrent` file
type Torrent struct {
	Announce *url.URL // announce URL of the tracker
	InfoHash []byte   // 20-byte long SHA1-hash of the bencode encoded info dictionary
	Mode     uint8    // enum specifying if single-file torrent or multi-file
	Files    []*File  // list of Files, where downloaded data needs to be written
	DirName  string   // name of the directory
	PieceLen uint32   // length of each piece in bytes (equal)
	Pieces   []*Piece // list containing pieces of data
}

// Read reads a bencode dictionary and populates
// all the fields of `Torrent` accordingly
func (t *Torrent) Read(dict *map[string]interface{}) error {
	var err error

	// reading the announce-url from bencode metainfo dictionary
	t.Announce, err = url.Parse((*dict)["announce"].(string))
	if err != nil {
		return err
	}

	// calculating infohash, a 20-byte long SHA1 hash of bencode encoded
	// info value. Every torrent is uniquely identified by its infohash
	enc := bencode.Encode((*dict)["info"])
	h := sha1.New()
	h.Write(enc)
	t.InfoHash = h.Sum(nil)

	// converting info into a dictionary (map[string]interface{})
	info := (*dict)["info"].(map[string]interface{})

	// extracting each piece length from
	// the decoded info dictionary
	t.PieceLen = uint32(info["piece length"].(int64))

	// concatinated SHA1 hash of all the pieces,
	// can be used to extract the number of pieces
	pieces := []byte(info["pieces"].(string))

	// reading pieces from the concatinated hash
	// and appending `*Piece` to the `Torrent`
	for i := 0; i+20 <= len(pieces); i += 20 {
		t.Pieces = append(t.Pieces, &Piece{
			Hash:  pieces[i : i+20],
			Index: uint32(i / 20),
		})
	}

	// checking if `info["files"]` property exists. If "yes" then
	// it's a multi file downloader, else single-file downloader
	if _, ok := info["files"]; ok {
		t.Mode = TorrMultiFile            // setting file-mode to multi-file enum
		t.DirName = info["name"].(string) // root directory name

		// converting the value at `info["files"]` into a list
		files := info["files"].([]interface{})

		for _, file := range files {
			// converting each element into dictionaries,
			// that describes a single file
			f := file.(map[string]interface{})

			// extracting the file path from the
			// list of file and directory names
			var fp string
			for _, p := range f["path"].([]interface{}) {
				fp = path.Join(fp, p.(string))
			}

			// appending all the files in `Piles` peroperty of `Torrent`
			t.Files = append(t.Files, &File{
				Path:   fp,
				Length: uint32(f["length"].(int64)),
			})
		}
	} else {
		t.Mode = TorrSingleFile // single-file mode

		// appending the single file in `Files` property.
		// for single-file mode length will always be 1
		t.Files = append(t.Files, &File{
			Path:   info["name"].(string),
			Length: uint32(info["length"].(int64)),
		})
	}

	return nil
}

// WriteBlock writes blocks of data recieved from
// other peers to the appropriate files
func (t *Torrent) WriteBlock(block *Block) error {
	return fmt.Errorf("not implemented")
}
