package main

import (
	"bytes"
	"fmt"
	"github.com/swatkat/gotrntmessages"
	"log"
	"math/big"
	"os"
	"runtime"
	"time"
)

// Piece data exchanged between peers and piecemgr
type PieceChunkData struct {
	peerInfo  *PeerInfo                   // Peer for this piece
	pieceInfo gotrntmessages.MsgDataPiece // Actual piece
}

// Piece download/upload manager
type PieceMgr struct {
	PieceWriterChan chan PieceChunkData // Incoming pieces downloaded from peers
	PieceMap        map[int64]uint32    // Piece index -> piece offset map
	Files           []*os.File          // List of file handles
}

// Start piecemgr
func (pieceMgr *PieceMgr) Start(sessionInfo *TrntSessionInfo) bool {
	// Sanity checks
	if sessionInfo == nil {
		log.Println(DebugGetFuncName(), "Invalid param")
	}

	fmt.Println(DebugGetFuncName(), "PieceMgr")
	pieceMgr.PieceWriterChan = make(chan PieceChunkData, 5)
	pieceMgr.PieceMap = make(map[int64]uint32)

	// Start torrenting
	go pieceMgr.pieceRequester(sessionInfo)
	go pieceMgr.pieceReceiver(sessionInfo)

	return true
}

func (pieceMgr *PieceMgr) Stop() bool {
	fmt.Println(DebugGetFuncName(), "Stop")
	return true
}

// Sends piece requests to peers
func (pieceMgr *PieceMgr) pieceRequester(sessionInfo *TrntSessionInfo) {
	// Plan:
	// 1. Find rare pieces
	// 2. Send Interested to all peers with rare pieces
	// 3. Wait for Unchoke message from peers
	// 4. Send request for pieces
	// 5. Repeat
	for {
		rarePieces := pieceMgr.findRarePieces(sessionInfo)
		for i := 0; i < rarePieces.BitLen(); i++ {
			if rarePieces.Bit(i) == 0 {
				continue
			}
			peerInfo, ok := pieceMgr.findPeerForPiece(sessionInfo, uint32(i))
			if !ok {
				continue
			}
			switch peerInfo.State {
			case PeerStateChoked:
				peerInfo.SendMsg(sessionInfo, gotrntmessages.MsgTypeInterested)

			case PeerStateUnchoked:
				pieceIdx := uint32(peerInfo.BitField.BitLen() - i)
				pieceOffset := uint32(0)
				pieceLen := uint32(trntCfg.PieceBlockLen)
				peerInfo.SendMsg(sessionInfo, gotrntmessages.MsgTypeRequest, pieceIdx, pieceOffset,
					pieceLen)
			}

			// Let peer go routines do their stuff now
			runtime.Gosched()
		}

		// We don't want to hog CPU
		runtime.Gosched()
	}

	return
}

// Build a bitfield containing rare pieces, that is pieces which very few peers have
func (pieceMgr *PieceMgr) findRarePieces(sessionInfo *TrntSessionInfo) *big.Int {
	rarePieces := big.NewInt(sessionInfo.peerMgr.myInfo.BitField.Int64())
	for _, val := range sessionInfo.peerMgr.peerMap {
		rarePieces.Xor(rarePieces, val.BitField)
	}
	return rarePieces
}

// Find a peer having required piece
func (pieceMgr *PieceMgr) findPeerForPiece(sessionInfo *TrntSessionInfo,
	pieceIdx uint32) (*PeerInfo, bool) {
	for _, val := range sessionInfo.peerMgr.peerMap {
		if val.BitField.Bit(int(pieceIdx)) != 0 {
			return val, true
		}
	}
	fmt.Println(DebugGetFuncName(), "No peers have piece:", pieceIdx)
	return nil, false
}

// Writes downloaded pieces to file
func (pieceMgr *PieceMgr) pieceReceiver(sessionInfo *TrntSessionInfo) {
	// Open files
	if !pieceMgr.openFiles(sessionInfo) {
		return
	}
	for _, f := range pieceMgr.Files {
		defer f.Close()
	}
	pieceMgr.loadPieceMap(sessionInfo)

	for {
		select {
		case chunkData := <-pieceMgr.PieceWriterChan:
			/*// Update piece map
			newChunkOffset := chunkData.pieceInfo.PieceBytesBegin + uint32(len(chunkData.pieceInfo.PieceBlock))
			pieceMgr.updatePieceMap(sessionInfo, int64(chunkData.pieceInfo.PieceIndex), newChunkOffset)

			// Update our own bitfield
			if newChunkOffset >= uint32(sessionInfo.metaInfo.Info.PieceLength) {
				idx := chunkData.peerInfo.BitField.BitLen() - int(chunkData.pieceInfo.PieceIndex)
				sessionInfo.peerMgr.myInfo.BitField.SetBit(sessionInfo.peerMgr.myInfo.BitField, idx, 1)
			}*/

			// Write to file
			fileByteOffset := (sessionInfo.metaInfo.Info.PieceLength *
				int64(chunkData.pieceInfo.PieceIndex)) + int64(chunkData.pieceInfo.PieceBytesBegin)
			bytesWritten, er := pieceMgr.Files[0].WriteAt(chunkData.pieceInfo.PieceBlock, fileByteOffset)
			if er != nil {
				log.Println(DebugGetFuncName(), er)
				continue
			}
			fmt.Println(DebugGetFuncName(), "Write to file, piece:",
				chunkData.pieceInfo.PieceIndex, ", offset:", fileByteOffset,
				", bytes written:", bytesWritten /*, ", data:", chunkData.pieceInfo.PieceBlock*/)

		case <-time.Tick(time.Nanosecond):
		}

		// Let other go routines do their stuff
		runtime.Gosched()
	}
}

// Open files on disk, don't close handles yet
func (pieceMgr *PieceMgr) openFiles(sessionInfo *TrntSessionInfo) bool {
	var er error
	var fileNames []string

	// Build a list of file names
	if len(sessionInfo.metaInfo.Info.Name) > 0 {
		fileNames = append(fileNames, sessionInfo.metaInfo.Info.Name)
	} else {
		for _, fileInfo := range sessionInfo.metaInfo.Info.Files {
			fileNames = append(fileNames, fileInfo.Path[0])
		}
	}

	if len(fileNames) <= 0 {
		log.Println(DebugGetFuncName(), "No files found")
		return false
	}

	// Open these files
	pieceMgr.Files = make([]*os.File, len(fileNames))
	for i, f := range fileNames {
		if pieceMgr.Files[i], er = os.Open(f); er != nil {
			pieceMgr.Files[i], er = os.Create(f)
		}
		if er != nil {
			for j := 0; j < i; j++ {
				pieceMgr.Files[j].Close()
			}
			log.Println(DebugGetFuncName(), er)
			return false
		}
	}
	return true
}

func (pieceMgr *PieceMgr) loadPieceMap(sessionInfo *TrntSessionInfo) bool {
	var zeroByte [1]byte
	zeroByte[0] = '0'
	pieceIdx := int64(0)
	buf := make([]byte, sessionInfo.metaInfo.Info.PieceLength)
	for _, f := range pieceMgr.Files {
		for {
			if _, er := f.Read(buf); er != nil {
				log.Println(DebugGetFuncName(), er)
				break
			}
			if pieceByteOffset := bytes.LastIndex(buf, zeroByte[0:]); pieceByteOffset >= 0 {
				pieceMgr.PieceMap[pieceIdx] = uint32(pieceByteOffset)
				fmt.Println(DebugGetFuncName(), "Piece index:", pieceIdx,
					", piece byte offset:", pieceByteOffset)
			}
			pieceIdx++
		}
	}
	return true
}

func (pieceMgr *PieceMgr) updatePieceMap(sessionInfo *TrntSessionInfo,
	pieceIdx int64, newChunkOffsetInPiece uint32) bool {
	pieceMgr.PieceMap[pieceIdx] = newChunkOffsetInPiece
	return true
}
