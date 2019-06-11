package torrent

import "fmt"

func Run(fp string) {
	torr, err := readfile(fp)
	if err != nil {
		panic(err)
	}

	url, err := gettrackerurl(torr)
	if err != nil {
		panic(err)
	}

	fmt.Println("url:", url)

}
