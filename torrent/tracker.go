package torrent

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"github.com/ritwik310/torrent-client/udp"
)

// geturlhttp generates a http url for tracker request including
// the necessary query params
func geturlhttp(torr *map[string]interface{}) (string, error) {
	// generating the url params for tracker request
	params := trackerparams(torr)

	// reading the url from torr data, torr["announce"]
	trkurl, err := url.Parse((*torr)["announce"].(string))
	if err != nil {
		fmt.Println("couldn't parse announce url, found in the torrent-file", err)
		return "", err
	}

	// appending required query params
	pr := url.Values{}
	for k, v := range params {
		pr.Add(k, v)
	}
	trkurl.RawQuery = pr.Encode()

	return trkurl.String(), nil
}

func trackerrequest(torr *map[string]interface{}) {
	ann, err := url.Parse((*torr)["announce"].(string))
	if err != nil {
		panic(err)
	}

	switch ann.Scheme {
	case "udp":
		fmt.Println("udp")

		protocol_id := make([]byte, 8)
		binary.BigEndian.PutUint64(protocol_id, 0x41727101980)

		action := make([]byte, 4)
		binary.BigEndian.PutUint32(action, 0)

		transaction_id := make([]byte, 4)
		binary.BigEndian.PutUint32(transaction_id, 908980) //random

		fmt.Println(len(protocol_id))
		fmt.Println(len(action))
		fmt.Println(len(transaction_id))

		var msg []byte
		msg = append(msg, protocol_id...)
		msg = append(msg, action...)
		msg = append(msg, transaction_id...)

		fmt.Println("^^^^^^^^^^", len(msg))

		client := udp.Client{
			Msg:  msg,
			Addr: ann.Host,
		}

		fmt.Println("announce:", ann.Host)

		err := trackerudp(&client)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println("Success! 1")

	case "http":
		fmt.Println("http")

		u, err := geturlhttp(torr)
		if err != nil {
			panic(err)
		}

		b, err := trackerhttp(u)
		if err != nil {
			panic(err)
		}

		fmt.Printf("Result:\n%s\n", b)
	}
}

func trackerudp(client *udp.Client) error {
	err := client.Send()
	if err != nil {
		return err
	}

	return nil
}

func trackerhttp(trkurl string) ([]byte, error) {
	resp, err := http.Get(trkurl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status-code not 200")
	}

	var body []byte
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return body, err
	}

	return body, nil
}
