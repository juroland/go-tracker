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
	IpAddress  uint32
	Key        uint32
	NumWant    int32
	Port       uint16
	Extension  uint16
}

type Peer struct {
	IPAddress int32
	TcpPort   int16
}

type IPv4AnnounceResponse struct {
	Action        int32
	TransactionID int32
	Interval      int32
	Leechers      int32
	Seeders       int32
	Peers         []Peer
}

func checkErr(err error) {
	if err != nil {
		log.Fatal("trackerudp: ", err)
	}
}

func handleRequest(conn *net.UDPConn) {
	buf := bytes.NewBuffer(make([]byte, MaxRequestSize))
	nread, client, err := conn.ReadFromUDP(buf.Bytes())
	if err != nil {
		log.Println("Request : read from udp failed : ", err)
		return
	}

	var header RequestHeader
	binary.Read(buf, binary.BigEndian, &header)
	fmt.Println(header)

	if header.Action == 0 {
		if header.ConnectionID != ProtocolID {
			log.Println("Request : wrong protocol id : ", header)
			return
		}
		var response ConnectResponse
		response.TransactionID = header.TransactionID
		buf.Reset()
		binary.Write(buf, binary.BigEndian, response)
		conn.WriteToUDP(buf.Bytes(), client)
		return
	}

	if header.Action == 1 {
		var req AnnounceRequest
		binary.Read(buf, binary.BigEndian, &req)
		fmt.Println(nread, req)
		return
	}
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
