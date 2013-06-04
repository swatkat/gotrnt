package main

import (
	"fmt"
	"log"
)

// Peer communication manager
type PeerMgr struct {
	peerMap map[string]*PeerInfo // Map of ip:port -> PeerInfo
	myInfo  PeerInfo             // Our info
}

// Start peermgr
func (peerMgr *PeerMgr) Start(sessionInfo *TrntSessionInfo) bool {
	// Sanity checks
	if sessionInfo == nil {
		log.Println(DebugGetFuncName(), "Invalid param")
		return false
	}

	fmt.Println(DebugGetFuncName(), "PeerMgr")

	// Init our state
	peerMgr.myInfo.Init("")

	// Loop through all peers obtained from tracker, and then
	// build a map of ip:port as key and PeerInfo struct as value
	peerMgr.peerMap = make(map[string]*PeerInfo)
	peerIpPortList := sessionInfo.trackerInfo.GetIpPortListFromPeers()
	for _, val := range peerIpPortList {
		peerInfo := new(PeerInfo)
		peerInfo.Init(val)
		peerMgr.peerMap[peerInfo.Addr] = peerInfo
	}

	// Loop through all peers and connect to them
	for _, val := range peerMgr.peerMap {
		val.Connect(sessionInfo)
	}

	return true
}

// Stop peermgr
func (peerMgr *PeerMgr) Stop() bool {
	// Loop through all peers of this session and disconnect them
	for _, val := range peerMgr.peerMap {
		val.Disconnect()
	}
	return true
}
