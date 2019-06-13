package torrent

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"

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

		client1 := connectionreq(ann.Host, 908980)

		by, err := trackerudp(&client1)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Println("len of by:", len(by))

		action := binary.BigEndian.Uint32(by[0:4])
		transaction_id := binary.BigEndian.Uint32(by[4:8])
		connection_id := binary.BigEndian.Uint64(by[8:16])

		fmt.Printf("haha - action:, %+v\n", action)
		fmt.Printf("haha - transaction_id:, %+v\n", transaction_id)
		fmt.Printf("haha - connection_id:, %+v\n", connection_id)

		client2 := announce(ann.Host, connection_id, transaction_id, torr)

		data, err := client2.Send()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		fmt.Printf("data:%v\n", len(data))

		fmt.Printf("action:%v\n", binary.BigEndian.Uint32(data[:4]))
		fmt.Printf("transaction_id:%v\n", binary.BigEndian.Uint32(data[4:8]))
		fmt.Printf("interval:%v\n", binary.BigEndian.Uint32(data[8:12]))
		fmt.Printf("leechers:%v\n", binary.BigEndian.Uint32(data[12:16]))
		// fmt.Printf("seeders:%v\n", binary.BigEndian.Uint32(data[16:20]))

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

func announce(addr string, connid uint64, tid uint32, torr *map[string]interface{}) udp.Client {
	params := trackerparams(torr)

	totalbytes, err := strconv.Atoi(params["left"])
	if err != nil {
		panic("Boom!")
	}

	connection_id := make([]byte, 8)
	action := make([]byte, 4)
	transaction_id := make([]byte, 4)
	info_hash := make([]byte, 20)
	peer_id := make([]byte, 20)
	downloaded := make([]byte, 8)
	left := make([]byte, 8)
	uploaded := make([]byte, 8)
	event := make([]byte, 4)
	ip := make([]byte, 4)
	key := make([]byte, 4)
	num_want := make([]byte, 4)
	port := make([]byte, 2)

	binary.BigEndian.PutUint64(connection_id, connid)
	binary.BigEndian.PutUint32(action, 1)
	binary.BigEndian.PutUint32(transaction_id, tid)
	info_hash = []byte(params["info_hash"])
	peer_id = []byte(params["peer_id"])
	binary.BigEndian.PutUint64(downloaded, 0)
	binary.BigEndian.PutUint64(left, uint64(totalbytes))
	binary.BigEndian.PutUint64(uploaded, 0)
	binary.BigEndian.PutUint32(event, 2)
	binary.BigEndian.PutUint32(ip, 0)
	binary.BigEndian.PutUint32(key, 12345)
	binary.BigEndian.PutUint32(num_want, 0)
	binary.BigEndian.PutUint16(port, 6888)

	var msg []byte
	msg = append(msg, connection_id...)
	msg = append(msg, action...)
	msg = append(msg, transaction_id...)
	msg = append(msg, info_hash...)
	msg = append(msg, peer_id...)
	msg = append(msg, downloaded...)
	msg = append(msg, left...)
	msg = append(msg, uploaded...)
	msg = append(msg, event...)
	msg = append(msg, ip...)
	msg = append(msg, key...)
	msg = append(msg, num_want...)
	msg = append(msg, port...)

	return udp.Client{
		Msg:  msg,
		Addr: addr,
	}
}

func connectionreq(addr string, tid uint32) udp.Client {
	protocolid := make([]byte, 8)
	binary.BigEndian.PutUint64(protocolid, 0x41727101980)

	action := make([]byte, 4)
	binary.BigEndian.PutUint32(action, 0)

	transactionid := make([]byte, 4)
	binary.BigEndian.PutUint32(transactionid, tid) //random

	var msg []byte
	msg = append(msg, protocolid...)
	msg = append(msg, action...)
	msg = append(msg, transactionid...)

	return udp.Client{
		Msg:  msg,
		Addr: addr,
	}
}

func trackerudp(client *udp.Client) ([]byte, error) {
	by, err := client.Send()
	// err := client.Send2()
	if err != nil {
		return nil, err
	}

	return by, nil
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
