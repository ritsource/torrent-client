package torrent

import (
	"fmt"
	"net/url"
)

func gettrackerurl(torr *map[string]interface{}) (string, error) {
	params := trackerparams(torr)

	trkurl, err := url.Parse((*torr)["announce"].(string))
	if err != nil {
		fmt.Println("couldn't parse announce url, found in the torrent-file", err)
		return "", err
	}

	pr := url.Values{}
	for k, v := range params {
		pr.Add(k, v)
	}

	trkurl.RawQuery = pr.Encode()

	return trkurl.String(), nil
}
