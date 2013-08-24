package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
)

var (
	nickRegexp    = regexp.MustCompile(`^[a-zA-Z\[\]_^{|}][a-zA-Z0-9\[\]_^{|}]*$`)
	channelRegexp = regexp.MustCompile(`^#[a-z0-9_\-]+$`)
)

func NewServer() *Server {
	return &Server{eventChan: make(chan Event),
		name:        "rosella",
		clientMap:   make(map[string]*Client),
		channelMap:  make(map[string]*Channel),
		operatorMap: make(map[string]string)}
}

func (s *Server) Run() {
	for event := range s.eventChan {
		s.handleEvent(event)
	}
}

func (s *Server) HandleConnection(conn net.Conn) {
	client := &Client{server: s,
		connection: conn,
		outputChan: make(chan string),
		signalChan: make(chan int, 3),
		channelMap: make(map[string]*Channel),
		connected:  true}

	go client.clientThread()
}

func (s *Server) handleEvent(e Event) {
	fields := strings.Fields(e.input)

	if len(fields) < 1 {
		return
	}

	if strings.HasPrefix(fields[0], ":") {
		fields = fields[1:]
	}

	command := strings.ToUpper(fields[0])
	args := fields[1:]

	switch {
	case command == "NICK":
		if len(args) < 1 {
			e.client.reply(errNoNick)
			return
		}

		newNick := args[0]

		//Check newNick is of valid formatting (regex)
		if nickRegexp.MatchString(newNick) == false {
			e.client.reply(errInvalidNick, newNick)
			return
		}

		if _, exists := s.clientMap[newNick]; exists {
			e.client.reply(errNickInUse, newNick)
			return
		}

		//Protect the server name from being used
		if newNick == s.name {
			e.client.reply(errNickInUse, newNick)
			return
		}

		e.client.setNick(newNick)

	case command == "USER":
		if e.client.nick == "" {
			e.client.reply(rplKill, "Your nickname is already being used")
			e.client.disconnect()
		} else {
			e.client.reply(rplWelcome)
			e.client.registered = true
		}

	case command == "JOIN":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		if args[0] == "0" {
			//Quit all channels
			for channel := range e.client.channelMap {
				e.client.partChannel(channel)
			}
			return
		}

		channels := strings.Split(args[0], ",")
		for _, channel := range channels {
			//Join the channel if it's valid
			if channelRegexp.Match([]byte(channel)) {
				e.client.joinChannel(channel)
			}
		}

	case command == "PART":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		channels := strings.Split(args[0], ",")
		for _, channel := range channels {
			//Part the channel if it's valid
			if channelRegexp.Match([]byte(channel)) {
				e.client.partChannel(channel)
			}
		}

	case command == "PRIVMSG":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 2 {
			e.client.reply(errMoreArgs)
			return
		}

		message := strings.Join(args[1:], " ")

		channel, chanExists := s.channelMap[args[0]]
		client, clientExists := s.clientMap[args[0]]

		if chanExists {
			for _, c := range channel.clientMap {
				if c != e.client {
					c.reply(rplMsg, e.client.nick, args[0], message)
				}
			}
		} else if clientExists {
			client.reply(rplMsg, e.client.nick, client.nick, message)
		} else {
			e.client.reply(errNoSuchNick, args[0])
		}

	case command == "QUIT":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		e.client.disconnect()

	case command == "TOPIC":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		channel, exists := s.channelMap[args[0]]
		if exists == false {
			e.client.reply(errNoSuchNick, args[0])
			return
		}

		channelName := args[0]

		if len(args) == 1 {
			e.client.reply(rplTopic, channelName, channel.topic)
			return
		}

		if args[1] == ":" {
			channel.topic = ""
			for _, client := range channel.clientMap {
				client.reply(rplNoTopic, channelName)
			}
		} else {
			topic := strings.Join(args[1:], " ")
			topic = strings.TrimPrefix(topic, ":")
			channel.topic = topic

			for _, client := range channel.clientMap {
				client.reply(rplTopic, channelName, channel.topic)
			}
		}

	case command == "LIST":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) == 0 {
			chanList := make([]string, 0, len(s.channelMap))

			for channelName, channel := range s.channelMap {
				listItem := fmt.Sprintf("%s %d :%s", channelName, len(channel.clientMap), channel.topic)
				chanList = append(chanList, listItem)
			}

			e.client.reply(rplList, chanList...)

		} else {
			channels := strings.Split(args[0], ",")
			chanList := make([]string, 0, len(channels))

			for _, channelName := range channels {
				if channel, exists := s.channelMap[channelName]; exists {
					listItem := fmt.Sprintf("%s %d :%s", channelName, len(channel.clientMap), channel.topic)
					chanList = append(chanList, listItem)
				}
			}

			e.client.reply(rplList, chanList...)
		}
	case command == "OPER":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 2 {
			e.client.reply(errMoreArgs)
			return
		}

		username := args[0]
		password := args[1]

		if hashedPassword, exists := s.operatorMap[username]; exists {
			h := sha1.New()
			io.WriteString(h, password)
			pass := fmt.Sprintf("%x", h.Sum(nil))
			if hashedPassword == pass {
				e.client.operator = true
				e.client.reply(rplOper)
				return
			}
		}
		e.client.reply(errPassword)

	case command == "KILL":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if e.client.operator == false {
			e.client.reply(errNoPriv)
			return
		}

		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		nick := args[0]

		if client, exists := s.clientMap[nick]; exists {
			client.reply(rplKill, "An operator has disconnected you.")
			client.disconnect()
		} else {
			e.client.reply(errNoSuchNick, nick)
		}

	default:
		e.client.reply(errUnknownCommand, command)
	}
}
