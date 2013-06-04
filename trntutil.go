package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// BitTorrent protocol header
var goTrntHeader string = "BitTorrent protocol"
var goTrntHeaderLen byte = byte(len(goTrntHeader))

// Stores gotrnt app specific config
type GoTorrentCfg struct {
	Port               uint16        // Port on which we listen for new peers
	PeerId             string        // Our peer id, randomly generated
	WaitForListener    chan bool     // Todo
	MyTCPAddr          *net.TCPAddr  // Our server port
	PeerConnectTimeout time.Duration // Timeout in seconds, used while connecting to peers
	PieceBlockLen      uint32        // Size of block in a piece, used while downloading a piece
}

// Global containing GoTrnt specific data
var trntCfg GoTorrentCfg

// Initialize gotrnt app specific config
func init() {
	trntCfg.PeerId = generatePeerId()
	trntCfg.Port = 6882
	trntCfg.WaitForListener = make(chan bool)
	str := fmt.Sprintf(":%d", trntCfg.Port)
	trntCfg.MyTCPAddr, _ = net.ResolveTCPAddr("tcp", str)
	trntCfg.PeerConnectTimeout = 2 * time.Second
	trntCfg.PieceBlockLen = 0x4000 // 16KB
	fmt.Println(DebugGetFuncName(), "My address: ", trntCfg.MyTCPAddr)
}

// Generate a 20 byte peer id for us
func generatePeerId() string {
	peerId := "GT" + strconv.Itoa(os.Getpid()) + "-" +
		strconv.FormatInt(rand.Int63(), 10)
	return peerId[0:20]
}

func DebugGetFuncName() string {
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		me := runtime.FuncForPC(pc)
		if me != nil {
			callerName := me.Name()
			lastIndex := strings.LastIndex(callerName, ".")
			if lastIndex >= 0 && (lastIndex+1) < len(callerName) {
				callerName = callerName[lastIndex+1:]
			}
			callerName += ":"
			return callerName
		}
	}
	return "UnknownFunc:"
}

func getBytesFromUint32(num uint32) []byte {
	var buf [4]byte
	buf[0] = byte((num >> 24) & 0xff)
	buf[1] = byte((num >> 16) & 0xff)
	buf[2] = byte((num >> 8) & 0xff)
	buf[3] = byte(num & 0xff)
	return buf[0:]
}

func getBytesFromUint16(num uint16) []byte {
	var buf [2]byte
	buf[0] = byte((num >> 8) | 0xff)
	buf[1] = byte(num | 0xff)
	return buf[0:]
}

func getUint16FromBytes(buf []byte) uint16 {
	if len(buf) < 2 {
		return 0
	}
	return ((uint16(buf[0]) << 8) | uint16(buf[1]))
}

func getUint32FromBytes(buf []byte) uint32 {
	if len(buf) < 4 {
		return 0
	}
	return ((uint32(buf[0]) << 24) | (uint32(buf[1]) << 16) |
		(uint32(buf[2]) << 8) | uint32(buf[3]))
}
