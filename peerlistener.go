package main

import (
	"fmt"
	"log"
	"net"
)

func StartGoTrntListener() bool {
	fmt.Println("=======================Starting listener=======================")
	go goTrntListener()
	return true
}

func goTrntListener() bool {
	tcpListener, er := net.ListenTCP("tcp", trntCfg.MyTCPAddr)
	if er != nil {
		log.Println(DebugGetFuncName(), er)
		return false
	}
	fmt.Println("=======================Listener started==================")
	peerConn, er := tcpListener.AcceptTCP()
	if er != nil {
		log.Println(DebugGetFuncName(), er)
	}
	fmt.Println("Accepted conn from ", peerConn)
	return true
}

func WaitForGoTrntListener() bool {
	fmt.Println(DebugGetFuncName(), "Waiting on listener chan")
	done := <-trntCfg.WaitForListener
	return done
}
