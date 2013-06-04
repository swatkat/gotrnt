package main

import (
	"fmt"
	"github.com/swatkat/gotrntmessages"
	"io"
	"log"
	"math/big"
	"net"
)

// Peer states
const (
	PeerStateChoked = iota
	PeerStateWaitForUnchoke
	PeerStateUnchoked
	PeerStateWaitForPiece
)

type PeerInfo struct {
	State        uint32   // Peer state
	IsInterested bool     // Interested or not
	Addr         string   // Peer ip:port
	Conn         net.Conn // Peer connection
	PeerId       string   // Peer id got from Handshake
	BitField     *big.Int // Bitfield indicating pices that a peer has
}

// Initalizes data related to peer state
func (peerInfo *PeerInfo) Init(peerIpPort string) {
	peerInfo.Addr = peerIpPort
	peerInfo.updateState(PeerStateChoked)
	peerInfo.IsInterested = false
	peerInfo.BitField = big.NewInt(0)
}

// Opens a TCP connection to peer
func (peerInfo *PeerInfo) Connect(sessionInfo *TrntSessionInfo) bool {
	// Sanity checks
	if sessionInfo == nil {
		log.Println(DebugGetFuncName(), "Invalid param")
		return false
	}

	// Connect to a peer
	var er error
	if peerInfo.Conn, er = net.DialTimeout("tcp", peerInfo.Addr,
		trntCfg.PeerConnectTimeout); er != nil {
		log.Println(DebugGetFuncName(), er)
		return false
	}

	// Start receiving msgs from peer
	go peerInfo.recvMsgs(sessionInfo)

	return true
}

// Disconnects from a peer
func (peerInfo *PeerInfo) Disconnect() bool {
	if er := peerInfo.Conn.Close(); er != nil {
		log.Println(DebugGetFuncName(), er)
	}
	peerInfo.Init("")
	return true
}

// Do handshake with peer and wait for msgs
func (peerInfo *PeerInfo) recvMsgs(sessionInfo *TrntSessionInfo) {
	// Send handshake
	if !peerInfo.SendMsg(sessionInfo, gotrntmessages.MsgTypeHandshake) {
		peerInfo.Disconnect()
		return
	}

	// Send our bitfield
	peerInfo.SendMsg(sessionInfo, gotrntmessages.MsgTypeBitfield)

	// First msg that we get from peer must be handshake
	buf := make([]byte, 68)
	if _, er := io.ReadFull(peerInfo.Conn, buf); er != nil {
		peerInfo.Disconnect()
		log.Println(DebugGetFuncName(), "Handshake:", er)
		return
	}
	msgData, ok := gotrntmessages.DecodeMessage(buf)
	if !ok || !peerInfo.ProcessMsg(sessionInfo, msgData) {
		peerInfo.Disconnect()
		log.Println(DebugGetFuncName(), "Error decoding handshake msg, peer:",
			peerInfo.Addr)
		return
	}

	// Process all other messages. Message format <len><id><payload>
	for {
		// Read length of the message
		var msglenbuf [4]byte
		if _, er := io.ReadFull(peerInfo.Conn, msglenbuf[0:]); er != nil {
			log.Println(DebugGetFuncName(), er)
			break
		}
		msglen := getUint32FromBytes(msglenbuf[0:])
		if msglen <= 0 {
			log.Println(DebugGetFuncName(), "Invalid msg len, peer:",
				peerInfo.Addr)
			continue
		}

		// Read rest of the message
		buf = make([]byte, msglen+4)
		if _, er := io.ReadFull(peerInfo.Conn, buf[4:]); er != nil {
			log.Println(DebugGetFuncName(), er)
			break
		}

		// Prefix msg len to the read message, and decode it
		copy(buf[0:4], msglenbuf[0:])
		msgData, ok = gotrntmessages.DecodeMessage(buf)

		// Process message and take action
		if !ok || !peerInfo.ProcessMsg(sessionInfo, msgData) {
			log.Println(DebugGetFuncName(), "Error decoding msg, peer:",
				peerInfo.Addr)
			continue
		}
	}

	// If we're here, then there's something wrong. Close connection
	peerInfo.Disconnect()
}

// Processes message received from a peer and actually takes action
func (peerInfo *PeerInfo) ProcessMsg(sessionInfo *TrntSessionInfo,
	msgBase gotrntmessages.MsgData) bool {
	// Convert to suitable struct based on message type
	msgType, _ := msgBase.GetMsgType()
	switch msgType {
	case gotrntmessages.MsgTypeChoke, gotrntmessages.MsgTypeUnchoke:
		msgData := msgBase.(gotrntmessages.MsgDataChoke)
		fmt.Println(DebugGetFuncName(), "Choke:", msgData.IsChoking, ", peer:",
			peerInfo.Addr)
		if msgData.IsChoking {
			peerInfo.updateState(PeerStateChoked)
		} else {
			peerInfo.updateState(PeerStateUnchoked)
		}

	case gotrntmessages.MsgTypeInterested, gotrntmessages.MsgTypeNotInterested:
		msgData := msgBase.(gotrntmessages.MsgDataInterested)
		fmt.Println(DebugGetFuncName(), "Interested:", msgData.IsInterested)
		peerInfo.IsInterested = msgData.IsInterested

	case gotrntmessages.MsgTypeHave:
		msgData := msgBase.(gotrntmessages.MsgDataHave)
		// big.Int stores bitfield in big endian format
		idx := peerInfo.BitField.BitLen() - int(msgData.PieceIndex)
		if (idx >= 0) && (peerInfo.BitField.Bit(idx) == 0) {
			peerInfo.BitField.SetBit(peerInfo.BitField, idx, 1)
			fmt.Println(DebugGetFuncName(), "Set bit index", msgData.PieceIndex,
				", peer:", peerInfo.Addr)
		}

	case gotrntmessages.MsgTypeBitfield:
		msgData := msgBase.(gotrntmessages.MsgDataBitfield)
		if len(msgData.Bitfield) <= 0 {
			log.Println(DebugGetFuncName(), "Invalid bitfield, peer:",
				peerInfo.Addr)
			return false
		}
		// Save bitfield for this peer
		peerInfo.BitField.SetBytes(msgData.Bitfield)

	case gotrntmessages.MsgTypeRequest, gotrntmessages.MsgTypeCancel:
		msgData := msgBase.(gotrntmessages.MsgDataRequestCancel)
		fmt.Println(DebugGetFuncName(), "Index:",
			msgData.PieceIndex, ", byte offset:", msgData.PieceBytesBegin,
			", byte len:", msgData.PieceBytesLen, ", peer:", peerInfo.Addr)

	case gotrntmessages.MsgTypePiece:
		msgData := msgBase.(gotrntmessages.MsgDataPiece)
		fmt.Println(DebugGetFuncName(), "Piece:", msgData.PieceIndex, "chunk offset:",
			msgData.PieceBytesBegin, ", peer:", peerInfo.Addr)
		if peerInfo.getState() == PeerStateWaitForPiece {
			peerInfo.updateState(PeerStateUnchoked)
		}
		// Push piece to piecemgr for writing into file
		var chunkData PieceChunkData
		chunkData.peerInfo = peerInfo
		chunkData.pieceInfo = msgData
		sessionInfo.pieceMgr.PieceWriterChan <- chunkData

	case gotrntmessages.MsgTypePort:
		msgData := msgBase.(gotrntmessages.MsgDataPort)
		fmt.Println(DebugGetFuncName(), "Port:", msgData.PeerPort, ", peer:",
			peerInfo.Addr)

	case gotrntmessages.MsgTypeHandshake:
		msgData := msgBase.(gotrntmessages.MsgDataHandshake)
		if sessionInfo.metaInfo.InfoHash != msgData.InfoHash {
			log.Println(DebugGetFuncName(), "Infohash mismatch, peer:",
				peerInfo.Addr)
			return false
		}
		peerInfo.PeerId = msgData.PeerId

	default:
		fmt.Println(DebugGetFuncName(), "Unknown msg:", msgType, ", peer:",
			peerInfo.Addr)
	}

	return true
}

func (peerInfo *PeerInfo) SendMsg(sessionInfo *TrntSessionInfo,
	msgType uint, v ...interface{}) bool {
	switch msgType {
	case gotrntmessages.MsgTypeInterested:
		if peerInfo.getState() == PeerStateChoked {
			if buf, ok := gotrntmessages.EncodeMessage(msgType, nil); ok {
				if peerInfo.send(msgType, buf) {
					peerInfo.updateState(PeerStateWaitForUnchoke)
					return true
				}
			}
		}

	case gotrntmessages.MsgTypeNotInterested:
		if buf, ok := gotrntmessages.EncodeMessage(msgType, nil); ok {
			return peerInfo.send(msgType, buf)
		}

	case gotrntmessages.MsgTypeHave:
		if len(v) == 1 {
			var msgData gotrntmessages.MsgDataHave
			msgData.MsgType = msgType
			msgData.PieceIndex = v[0].(uint32) // piece index
			if buf, ok := gotrntmessages.EncodeMessage(msgType, msgData); ok {
				return peerInfo.send(msgType, buf)
			}
		} else {
			log.Println(DebugGetFuncName(), "Invalid arg, len:", len(v),
				", msg:", gotrntmessages.MsgTypeNames[msgType], ", peer:", peerInfo.Addr)
		}

	case gotrntmessages.MsgTypeBitfield:
		if sessionInfo.peerMgr.myInfo.BitField.BitLen() > 0 {
			var msgData gotrntmessages.MsgDataBitfield
			msgData.MsgType = msgType
			msgData.Bitfield = sessionInfo.peerMgr.myInfo.BitField.Bytes()
			if buf, ok := gotrntmessages.EncodeMessage(msgType, msgData); ok {
				return peerInfo.send(msgType, buf)
			}
		}

	case gotrntmessages.MsgTypeRequest:
		if len(v) == 3 {
			var msgData gotrntmessages.MsgDataRequestCancel
			msgData.MsgType = msgType
			msgData.PieceIndex = v[0].(uint32)      // piece index
			msgData.PieceBytesBegin = v[1].(uint32) // piece begin
			msgData.PieceBytesLen = v[2].(uint32)   // piece len
			if buf, ok := gotrntmessages.EncodeMessage(msgType, msgData); ok {
				if peerInfo.send(msgType, buf) {
					peerInfo.updateState(PeerStateWaitForPiece)
					return true
				}
			}
		} else {
			log.Println(DebugGetFuncName(), "Invalid arg, len:", len(v),
				", msg:", gotrntmessages.MsgTypeNames[msgType], ", peer:", peerInfo.Addr)
		}

	case gotrntmessages.MsgTypeHandshake:
		var msgData gotrntmessages.MsgDataHandshake
		msgData.MsgType = msgType
		msgData.PeerId = trntCfg.PeerId
		msgData.InfoHash = sessionInfo.metaInfo.InfoHash
		if buf, ok := gotrntmessages.EncodeMessage(msgType, msgData); ok {
			return peerInfo.send(msgType, buf)
		}

	default:
		fmt.Println(DebugGetFuncName(), "Unknown msg:", msgType, ", peer:",
			peerInfo.Addr)
	}

	return false
}

// Generic method to send a message to peer; message format is raw bytes
func (peerInfo *PeerInfo) send(msgType uint, buf []byte) bool {
	// Sanity checks
	if len(buf) == 0 {
		log.Println(DebugGetFuncName(), "Invalid msg length, to peer:",
			peerInfo.Addr)
		return false
	}

	// Write to socket	
	fmt.Println(DebugGetFuncName(), "Sending:", gotrntmessages.MsgTypeNames[msgType], ", peer:",
		peerInfo.Addr)
	if _, er := peerInfo.Conn.Write(buf); er != nil {
		log.Println(DebugGetFuncName(), er, ", peer:", peerInfo.Addr)
		return false
	}
	return true
}

// Get peer's current state
func (peerInfo *PeerInfo) getState() uint32 {
	return peerInfo.State
}

// Update peer's state based on messages processed
func (peerInfo *PeerInfo) updateState(newState uint32) {
	peerInfo.State = newState
}
