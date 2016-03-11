package main

import (
	"net"
	"sync"
)

const (
	VERSION = "1.1.1"
)

type Server struct {
	eventChan   chan Event
	running     bool
	name        string
	clientMap   map[string]*Client  //Map of nicks → clients
	channelMap  map[string]*Channel //Map of channel names → channels
	operatorMap map[string]string   //Map of usernames → SHA1 hashed passwords
	motd        string
}

type Client struct {
	server     *Server
	connection net.Conn
	stopChan   chan struct{}
	outputChan chan string
	nick       string
	key        string
	registered bool
	operator   bool
	channelMap map[string]*Channel
	lock       sync.Mutex
}

type eventType int

const (
	connected eventType = iota
	disconnected
	command
)

type Event struct {
	client *Client
	input  string
	event  eventType
}

type Channel struct {
	name      string
	topic     string
	clientMap map[string]*Client
	mode      ChannelMode
	modeMap   map[string]*ClientMode
}

type ChannelMode struct {
	secret      bool //Channel is hidden from LIST
	topicLocked bool //Only ops may change topic
	moderated   bool //Only ops and voiced may speak
	noExternal  bool //Only users in the channel may talk to it
}

func (m *ChannelMode) String() string {
	modeStr := ""
	if m.secret {
		modeStr += "s"
	}
	if m.topicLocked {
		modeStr += "t"
	}
	if m.moderated {
		modeStr += "m"
	}
	if m.noExternal {
		modeStr += "n"
	}
	return modeStr
}

type ClientMode struct {
	operator bool //Channel operator
	voice    bool //Has voice
}

func (m *ClientMode) Prefix() string {
	if m.operator {
		return "@"
	} else if m.voice {
		return "+"
	} else {
		return ""
	}
}

func (m *ClientMode) String() string {
	modeStr := ""
	if m.operator {
		modeStr += "o"
	}
	if m.voice {
		modeStr += "v"
	}
	return modeStr
}

type replyCode int

const (
	rplWelcome replyCode = iota
	rplJoin
	rplPart
	rplTopic
	rplNoTopic
	rplNames
	rplEndOfNames
	rplNickChange
	rplKill
	rplMsg
	rplList
	rplOper
	rplChannelModeIs
	rplKick
	rplInfo
	rplVersion
	rplMOTD
	rplPong
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
	errCannotSend
)
