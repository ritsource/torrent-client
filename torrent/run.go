package torrent

func Run(fp string) {
	torr, err := readfile(fp)
	if err != nil {
		panic(err)
	}

	trackerrequest(torr)
	return

}
