gotrnt
======
Simple torrent library written in Go.

Work in progress. Does following as of now:
* Open and parse a torrent metainfo file (.torrent file)
* Send HTTP request to tracker got from announce URL in metainfo file
* Connect to peers returned by tracker
* Send handshake message to peers
* Listen for messages from these peers

Immediate todo:
* Download pieces
* Accept connections from neer peers
* Seed pieces

Build
=====
    go get code.google.com/p/bencode-go
    go get github.com/swatkat/gotrntmetainfoparser
    go get github.com/swatkat/gotrnttrackerquery
    go get github.com/swatkat/gotrntmessages
    go build

Run
=====
    gotrnt file.torrent

Info
=====
* peermgr.go and peer.go: Peer states and communication management
* piecemgr.go: Piece download logic
* trntsession.go: Reads torrent metainfo file, gets data from tracker and kick starts peermgr and piecemgr
