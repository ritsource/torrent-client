package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/marksamman/bencode"
	"github.com/sirupsen/logrus"
)

// Torr holds teh values read from
// the provided `.torrent` file
var Torr = &Torrent{}

// ReadFile reads the `.torrent` file provided
// in the arguemet and populates `Torr` with
// the relevent data. The torrent specific
// data is accessable via `src.Torr`
func ReadFile(fn string) error {

	// reading the `.torrent` file content. It
	// contains information about the files you
	// wanna download and where to find the
	// tracker, in a bencode dictionary
	f, err := os.Open(fn)
	if err != nil {
		logrus.Errorf("unable to read the `.torrent` file")
		return err
	}
	defer f.Close()

	// decoding bencode dictionary into a `map[string]interface{}`
	dict, err := bencode.Decode(f)
	if err != nil {
		logrus.Errorf("unable to decode the `.torrent` file")
		return err
	}

	// populating `Torr` with data
	// read from the decoded dictionary
	return Torr.Read(&dict)
}

// File holds value for every file to be downloaded
type File struct {
	Path   string // the path where the file needs to be written
	Start  int
	Length int // size of the file (in bytes)
}

// IsExist .
func (f *File) IsExist() (bool, error) {
	if _, err := os.Stat(f.Path); err == nil {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// WriteData .
func (f *File) WriteData(bs []byte, off int) (int, error) {
	exist, err := f.IsExist()
	if err != nil {
		return 0, err
	}

	var file *os.File
	if exist {
		err := os.MkdirAll(filepath.Dir(f.Path), os.ModePerm)
		if err != nil {
			return 0, err
		}
		file, err = os.Create(f.Path)
	} else {
		file, err = os.Open(f.Path)
	}

	if err != nil {
		return 0, err
	}
	defer file.Close()

	return file.WriteAt(bs, int64(off))
}

// Constants corrosponding to `Status` enum value of of `Piece`
const (
	PieceStatusDefault    uint8 = 0 // default state, 0 when a piece is created
	PieceStatusRequested  uint8 = 1 // when the piece have been requested to a peer
	PieceStatusDownloaded uint8 = 2 // when the piece download has successfully been completed
	PieceStatusFailed     uint8 = 3 // when the piece download has not been successful (failed atleast once)
)

// Piece represents an individual piece of data
type Piece struct {
	Index  uint32   // piece-index
	Hash   []byte   // 20-byte long SHA1-hash of the piece-data, extracted from `.torrent` file
	Length uint32   //  size of piece
	Blocks []*Block // blocks
	Status uint8    // status of the piece - exist (default), requested, downloaded, failed
}

// WriteToFiles .
func (p *Piece) WriteToFiles(data []byte) (int, error) {

	fs := Torr.WhichFiles(int(p.Index))

	// lastend := 0

	// `psoff` and `peoff` are the piece start offset and
	// piece end offset in the full concatinated data
	psoff := int(p.Length * p.Index)
	peoff := psoff + int(p.Length)

	for _, f := range fs {
		var ws int
		var we int
		var off int

		if f.Start > psoff {
			ws = f.Start
			off = 0
		} else {
			ws = psoff
			off = psoff - f.Start
		}

		if f.Start+f.Length > peoff {
			we = peoff
		} else {
			we = f.Start + f.Length
		}

		_, err := f.WriteData(data[ws:we], off)
		if err != nil {
			return 0, err
		}
	}

	fmt.Printf("%+v\n", fs[0])

	// os.Exit(0
	return int(p.Length), nil

}

// fileExist .
func fileExist(fpth string) bool {
	_, err := os.Stat(fpth)
	return err != nil && os.IsNotExist(err)
}

// GenBlocks .
func (p *Piece) GenBlocks() {
	n := int(math.Ceil(float64(p.Length / uint32(LengthOfBlock))))

	for i := 0; i < n; i++ {
		var ln int
		if i == n-1 && int(p.Length)%LengthOfBlock != 0 {
			ln = int(p.Length) % LengthOfBlock
		} else {
			ln = LengthOfBlock
		}

		p.Blocks = append(p.Blocks, &Block{
			PieceIndex: p.Index,
			Begin:      uint32(i * LengthOfBlock),
			Length:     uint32(ln),
		})
	}
}

/*
LengthOfBlock is the length of each block. While downloading pieces
from the peers, we request pieces in chunks. This is called a block.
Typically, each block happens to be 2^14 (16384) bytes in size
*/
var LengthOfBlock = int(math.Pow(2, 14))

// Block represents a block of data (a chunk of piece)
type Block struct {
	PieceIndex uint32 // piece-index of the piece that the block is a part of
	Begin      uint32 // offset where the block starts within the piece (that it's a part of)
	Length     uint32 // length of the block in bytes
}

// requestMsgBuf .
func (b *Block) requestMsgBuf() (*bytes.Buffer, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint32(13))
	err = binary.Write(buf, binary.BigEndian, uint8(6))     // id - request message
	err = binary.Write(buf, binary.BigEndian, b.PieceIndex) // piece index
	err = binary.Write(buf, binary.BigEndian, b.Begin)      //
	err = binary.Write(buf, binary.BigEndian, b.Length)

	return buf, err
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
	Size     int      // total size
	PFMap    [][]*File
}

// WhichFiles .
func (t *Torrent) WhichFiles(pidx int) []*File {
	return t.PFMap[pidx]
}

// PRFMap -> Piece Range / File map

// GenPFMap .
func (t *Torrent) GenPFMap() {
	mmap := make([][]*File, t.Size/int(t.PieceLen))

	for _, f := range t.Files {
		// fpof, first piece's index of this file
		// lpof, last piece's index of this file
		fpof, lpof := t.getFileOffset(f)

		for x := fpof; x <= lpof; x++ {
			mmap[int(x)] = append(mmap[int(x)], f)
		}
	}

	t.PFMap = mmap
}

func (t *Torrent) getFileOffset(f *File) (int, int) {
	s := int(math.Ceil(float64(f.Start / int(t.PieceLen))))
	e := int(math.Ceil(float64((f.Start + f.Length) / int(t.PieceLen))))

	if (f.Start+f.Length)%int(t.PieceLen) == 0 {
		return s, e - 1
	}
	return s, e

}

// Read reads a bencode dictionary and populates
// all the fields of `Torrent` accordingly
func (t *Torrent) Read(dict *map[string]interface{}) error {
	var err error

	// reading the announce-url from bencode metainfo dictionary
	// fmt.Println((*dict)["announce"].(string))
	t.Announce, err = url.Parse((*dict)["announce"].(string))
	if err != nil {
		return err
	}

	// calculating infohash, a 20-byte long SHA1 hash of bencode encoded
	// info value. Every torrent is uniquely identified by its infohash
	enc := bencode.Encode((*dict)["info"])
	hash, err := GetSHA1(enc)
	if err != nil {
		return err
	}
	t.InfoHash = hash

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
			Hash:   pieces[i : i+20],
			Index:  uint32(i / 20),
			Length: t.PieceLen,
		})
	}

	// total size of the content, to be downloaded
	t.Size = int(t.PieceLen) * len(t.Pieces)

	// checking if `info["files"]` property exists. If "yes" then
	// it's a multi file downloader, else single-file downloader
	if _, ok := info["files"]; ok {
		t.Mode = TorrMultiFile            // setting file-mode to multi-file enum
		t.DirName = info["name"].(string) // root directory name

		// converting the value at `info["files"]` into a list
		files := info["files"].([]interface{})
		dirnm := info["name"].(string)

		off := 0

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

			lng := int(uint32(f["length"].(int64)))

			// appending all the files in `Piles` peroperty of `Torrent`
			t.Files = append(t.Files, &File{
				Path:   path.Join(dirnm, fp),
				Start:  off,
				Length: lng,
			})

			off += lng
		}
	} else {
		t.Mode = TorrSingleFile // single-file mode

		// appending the single file in `Files` property.
		// for single-file mode length will always be 1
		t.Files = append(t.Files, &File{
			Path:   info["name"].(string),
			Start:  0,
			Length: int(uint32(info["length"].(int64))),
		})
	}

	return nil
}
