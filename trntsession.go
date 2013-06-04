package main

import (
	"github.com/swatkat/gotrntmetainfoparser"
	"github.com/swatkat/gotrnttrackerquery"
	"log"
)

type TrntSessionInfo struct {
	metaInfo    gotrntmetainfoparser.MetaInfo      // Torrent metafile content
	trackerInfo gotrnttrackerquery.TrackerResponse // Tracker response
	peerMgr     PeerMgr                            // Peer communication manager
	pieceMgr    PieceMgr                           // Manages downloading and seeding pieces
}

// Read .torrent file, send request to tracker and get a list of peers
func (sessionInfo *TrntSessionInfo) Init(fileNameWithPath string) bool {
	// Read torrent file
	if !sessionInfo.metaInfo.ReadTorrentMetaInfoFile(fileNameWithPath) {
		log.Println(DebugGetFuncName(), "Failed to read torrent file")
		return false
	}
	sessionInfo.metaInfo.DumpTorrentMetaInfo()

	// Get tracker info for this torrent
	if !sessionInfo.trackerInfo.GetTrackerInfo(sessionInfo.metaInfo.Announce,
		sessionInfo.metaInfo.InfoHash, trntCfg.PeerId, uint64(trntCfg.Port)) {
		log.Println(DebugGetFuncName(), "Failed to get tracker response")
		return false
	}
	sessionInfo.trackerInfo.DumpTrackerResponse()

	return true
}

// Start torrenting
func (sessionInfo *TrntSessionInfo) Start() bool {

	// Kick start peer mgr
	sessionInfo.peerMgr.Start(sessionInfo)

	// Kick start piece mgr
	sessionInfo.pieceMgr.Start(sessionInfo)

	return true
}

// Stop torrenting
func (sessionInfo *TrntSessionInfo) Stop() bool {

	// Stop peer mgr
	sessionInfo.peerMgr.Stop()

	// Stop piece mgr
	sessionInfo.pieceMgr.Stop()

	return true
}
