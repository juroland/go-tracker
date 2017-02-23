package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
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

var Seeders = make(map[[20]byte]Peer)
var Leechers = make(map[[20]byte]Peer)

func handleRequest(conn *net.UDPConn) {
	buf := bytes.NewBuffer(make([]byte, MaxRequestSize))
	_, client, err := conn.ReadFromUDP(buf.Bytes())
	if err != nil {
		log.Println("Request : read from udp failed : ", err)
		return
	}

	var header RequestHeader
	binary.Read(buf, binary.BigEndian, &header)
	fmt.Println(header)

	switch Action(header.Action) {
	case Connect:
		handleConnect(conn, client, &header, buf)
	case Announce:
		handleAnnounce(conn, client, &header, buf)
	}
}

func handleConnect(conn *net.UDPConn, client *net.UDPAddr, header *RequestHeader, buf *bytes.Buffer) {
	if header.ConnectionID != ProtocolID {
		log.Println("Request : wrong protocol id : ", header)
		return
	}

	var response ConnectResponse
	response.TransactionID = header.TransactionID
	buf.Reset()
	binary.Write(buf, binary.BigEndian, response)
	conn.WriteToUDP(buf.Bytes(), client)
}

func handleAnnounce(conn *net.UDPConn, client *net.UDPAddr, header *RequestHeader, buf *bytes.Buffer) {
	var req AnnounceRequest
	binary.Read(buf, binary.BigEndian, &req)

	peer := Peer{IPAddress: req.IPAddress, TCPPort: req.Port}
	if peer.IPAddress == 0 {
		peer.IPAddress = binary.BigEndian.Uint32(client.IP.To4())
	}
	log.Println("Announce request from : ", peer)

	switch Event(req.Event) {
	case Started:
		if req.Left > 0 {
			Leechers[req.PeerID] = peer
		} else {
			Seeders[req.PeerID] = peer
		}
	case Completed:
		delete(Seeders, req.PeerID)
		Seeders[req.PeerID] = peer
	case Stopped:
		delete(Seeders, req.PeerID)
		delete(Leechers, req.PeerID)
	}

	var response IPv4AnnounceResponseHeader
	response.Action = int32(Announce)
	response.TransactionID = header.TransactionID
	response.Interval = 30
	response.Leechers = int32(len(Leechers))
	response.Seeders = int32(len(Seeders))

	buf.Reset()
	binary.Write(buf, binary.BigEndian, response)

	for _, peer := range Leechers {
		binary.Write(buf, binary.BigEndian, peer)
	}

	for _, peer := range Seeders {
		binary.Write(buf, binary.BigEndian, peer)
	}

	conn.WriteToUDP(buf.Bytes(), client)
}

func main() {
	addr, err := net.ResolveUDPAddr("udp4", ":22222")
	checkErr(err)

	conn, err := net.ListenUDP("udp4", addr)
	checkErr(err)

	for {
		handleRequest(conn)
	}
}
