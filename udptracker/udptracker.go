package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
)

const ProtocolID = 0x41727101980
const MaxRequestSize = 100
const HeaderSize = 16

type Event int32

const (
	None Event = iota
	Completed
	Started
	Stopped
)

type Action int32

const (
	Connect Action = iota
	Announce
	Scrape
	Error
)

type RequestHeader struct {
	ConnectionID  int64
	Action        int32
	TransactionID int32
}

type ConnectResponse struct {
	Action        int32
	TransactionID int32
	ConnectionID  int64
}

type AnnounceRequest struct {
	InfoHash   [20]byte
	PeerID     [20]byte
	Downloaded int64
	Left       int64
	Uploaded   int64
	Event      int32
	IPAddress  uint32
	Key        uint32
	NumWant    int32
	Port       uint16
	Extension  uint16
}

type Peer struct {
	IPAddress uint32
	TCPPort   uint16
}

type IPv4AnnounceResponseHeader struct {
	Action        int32
	TransactionID int32
	Interval      int32
	Leechers      int32
	Seeders       int32
}

func checkErr(err error) {
	if err != nil {
		log.Fatal("trackerudp: ", err)
	}
}

var Seeders = make(map[string]Peer)
var Leechers = make(map[string]Peer)

type ResponseWriter struct {
	conn *net.UDPConn
	addr *net.UDPAddr
}

func (w ResponseWriter) Write(p []byte) (int, error) {
	w.conn.WriteToUDP(p, w.addr)
	return len(p), nil
}

type Request struct {
	message []byte
	ip      net.IP
}

func handleRequest(w io.Writer, req Request) {
	var header RequestHeader
	reader := bytes.NewReader(req.message)
	binary.Read(reader, binary.BigEndian, &header)

	fmt.Println(header)

	switch Action(header.Action) {
	case Connect:
		handleConnect(w, req, &header)
	case Announce:
		handleAnnounce(w, req, &header)
	}
}

func handleConnect(w io.Writer, req Request, header *RequestHeader) {
	if header.ConnectionID != ProtocolID {
		log.Println("Request : wrong protocol id : ", header)
		return
	}

	var response ConnectResponse
	response.TransactionID = header.TransactionID
	binary.Write(w, binary.BigEndian, response)
}

func handleAnnounce(w io.Writer, req Request, header *RequestHeader) {
	announce := &AnnounceRequest{}
	reader := bytes.NewReader(req.message[HeaderSize:])
	binary.Read(reader, binary.BigEndian, announce)

	peer := Peer{IPAddress: announce.IPAddress, TCPPort: announce.Port}
	if peer.IPAddress == 0 {
		peer.IPAddress = binary.LittleEndian.Uint32(req.ip)
	}
	log.Println("Announce request from : ", peer)

	peerID := string(announce.PeerID[:])
	switch Event(announce.Event) {
	case Started:
		if announce.Left > 0 {
			Leechers[peerID] = peer
		} else {
			Seeders[peerID] = peer
		}
	case Completed:
		delete(Seeders, peerID)
		Seeders[peerID] = peer
	case Stopped:
		delete(Seeders, peerID)
		delete(Leechers, peerID)
	}

	writeAnnounceResponse(w, header, announce)
}

func writeAnnounceResponse(w io.Writer, header *RequestHeader, announce *AnnounceRequest) {
	var response IPv4AnnounceResponseHeader
	response.Action = int32(Announce)
	response.TransactionID = header.TransactionID
	response.Interval = 30
	response.Leechers = int32(len(Leechers))
	response.Seeders = int32(len(Seeders))

	var buf bytes.Buffer
	binary.Write(&buf, binary.BigEndian, response)

	for _, peer := range Leechers {
		binary.Write(&buf, binary.BigEndian, peer)
	}

	for _, peer := range Seeders {
		binary.Write(&buf, binary.BigEndian, peer)
	}

	w.Write(buf.Bytes())
}

func main() {
	addr, err := net.ResolveUDPAddr("udp4", ":22222")
	checkErr(err)

	conn, err := net.ListenUDP("udp4", addr)
	checkErr(err)

	for {
		b := make([]byte, MaxRequestSize)
		_, clientAddr, err := conn.ReadFromUDP(b)
		if err != nil {
			log.Println("Request : read from udp failed : ", err)
			return
		}
		w := ResponseWriter{conn: conn, addr: clientAddr}

		req := Request{ip: clientAddr.IP.To4(), message: b}
		handleRequest(w, req)
	}
}
