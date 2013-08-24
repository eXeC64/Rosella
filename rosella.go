package main

import "net"

type Server struct {
	eventChan   chan Event
	running     bool
	name        string
	clientMap   map[string]*Client  //Map of nicks → clients
	channelMap  map[string]*Channel //Map of channel names → channels
	operatorMap map[string]string   //Map of usernames → SHA1 hashed passwords
}

type Client struct {
	server     *Server
	connection net.Conn
	signalChan chan signalCode
	outputChan chan string
	nick       string
	registered bool
	connected  bool
	operator   bool
	channelMap map[string]*Channel
}

type Event struct {
	client *Client
	input  string
}

type Channel struct {
	name      string
	topic     string
	clientMap map[string]*Client
}

type signalCode int

const (
	signalStop signalCode = iota
)

type replyCode int

const (
	rplWelcome replyCode = iota
	rplJoin
	rplPart
	rplTopic
	rplNoTopic
	rplNames
	rplNickChange
	rplKill
	rplMsg
	rplList
	rplOper
	errMoreArgs
	errNoNick
	errInvalidNick
	errNickInUse
	errAlreadyReg
	errNoSuchNick
	errUnknownCommand
	errNotReg
	errPassword
	errNoPriv
)
