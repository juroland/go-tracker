package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type Event int

const (
	started Event = iota
	stopped
	completed
	empty
)

func (e Event) String() string {
	switch e {
	case started:
		return "started"
	case stopped:
		return "stopped"
	case completed:
		return "comleted"
	default:
		return "unknown event"
	}
}

type Request struct {
	info_hash  []byte
	peer_id    string
	port       int
	uploaded   int
	downloaded int
	left       int
	event      Event
	compact    bool
}

type Peer struct {
	peer_id string
	ip      string
	port    int
}

type Response struct {
	interval int
	peers    []Peer
}

func ParseQueryKeyInt(query url.Values, key string) (int, error) {
	values, ok := query[key]
	if !ok {
		return 0, fmt.Errorf("bittorent: parse query: %s is missing", key)
	}

	value, err := strconv.Atoi(values[0])
	if err != nil {
		return 0, err
	}

	return value, nil
}

func ParseQuery(query url.Values) (r *Request, err error) {
	r = new(Request)

	info_hash, ok := query["info_hash"]
	if !ok {
		return nil, errors.New("bittorent: parse query: info_hash is missing")
	}

	r.info_hash = []byte(info_hash[0])

	peer_id, ok := query["peer_id"]
	if !ok {
		return nil, errors.New("bittorent: parse query: peer_id is missing")
	}
	r.peer_id = peer_id[0]

	r.port, err = ParseQueryKeyInt(query, "port")
	if err != nil {
		return nil, err
	}

	r.uploaded, err = ParseQueryKeyInt(query, "uploaded")
	if err != nil {
		return nil, err
	}

	r.downloaded, err = ParseQueryKeyInt(query, "downloaded")
	if err != nil {
		return nil, err
	}

	r.left, err = ParseQueryKeyInt(query, "left")
	if err != nil {
		return nil, err
	}

	event, ok := query["event"]
	if !ok {
		r.event = empty
	} else if event[0] == "started" {
		r.event = started
	} else if event[0] == "stopped" {
		r.event = stopped
	} else if event[0] == "completed" {
		r.event = completed
	}

	compact, ok := query["compact"]
	if ok && compact[0] == "1" {
		r.compact = true
	}

	return
}

var peers map[string]Peer

func writeCompactPeers(w http.ResponseWriter) {
	io.WriteString(w, fmt.Sprintf("%d:", len(peers)*6))
	for _, peer := range peers {
		ip := net.ParseIP(peer.ip)
		binary.Write(w, binary.BigEndian, ip.To4())
		port := int16(peer.port)
		binary.Write(w, binary.BigEndian, port)
	}
}

func writePeers(w http.ResponseWriter) {
	io.WriteString(w, "l")
	io.WriteString(w, fmt.Sprintf("%d:", len(peers)*6))
	for _, peer := range peers {
		io.WriteString(w, "d")
		io.WriteString(w, "2:id")
		io.WriteString(w, fmt.Sprintf("%d:%s", len(peer.peer_id), peer.peer_id))
		io.WriteString(w, fmt.Sprintf("2:ip%d:%s", peer.ip))
		io.WriteString(w, "4:port")
		io.WriteString(w, fmt.Sprintf("i%de", peer.port))
		io.WriteString(w, "e")
	}
	io.WriteString(w, "e")
}

func writeResponse(w http.ResponseWriter, compact bool) {
	if len(peers) == 0 {
		return
	}

	w.Header().Set("Content-Type", "text/plain")

	io.WriteString(w, "d")
	io.WriteString(w, "8:intervali30e")
	io.WriteString(w, "5:peers")

	if compact {
		writeCompactPeers(w)
	} else {
		writePeers(w)
	}

	io.WriteString(w, "e")
}

func Tracker(w http.ResponseWriter, req *http.Request) {
	log.Println(req.URL)

	query := req.URL.Query()
	r, err := ParseQuery(query)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(r)

	ip, _, _ := net.SplitHostPort(req.RemoteAddr)

	if r.event == stopped {
		log.Printf("Delete %v from peers.\n", r.peer_id)
		delete(peers, r.peer_id)
		return
	}

	if r.left == 0 {
		peers[r.peer_id] = Peer{r.peer_id, ip, r.port}
		log.Printf("Add %v to peers.\n", peers[r.peer_id])
	}

	writeResponse(w, r.compact)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: tracker path")
		os.Exit(1)
	}

	peers = make(map[string]Peer)
	http.HandleFunc(os.Args[1], Tracker)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
