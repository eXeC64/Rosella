package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strings"
)

var (
	nickRegexp    = regexp.MustCompile(`^[a-zA-Z\[\]_^{|}][a-zA-Z0-9\[\]_^{|}]*$`)
	channelRegexp = regexp.MustCompile(`^#[a-zA-Z0-9_\-]+$`)
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
		signalChan: make(chan signalCode, 3),
		channelMap: make(map[string]*Channel),
		connected:  true}

	go client.clientThread()
}

func (s *Server) handleEvent(e Event) {
	defer func(event Event) {
		err := recover()
		if err != nil {
			log.Printf("Recovered from errer when handling event: %+v", event)
			log.Println(err)
		}
	}(e)
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

		if _, exists := s.clientMap[strings.ToLower(newNick)]; exists {
			e.client.reply(errNickInUse, newNick)
			return
		}

		//Protect the server name from being used
		if strings.ToLower(newNick) == strings.ToLower(s.name) {
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

		channel, chanExists := s.channelMap[strings.ToLower(args[0])]
		client, clientExists := s.clientMap[strings.ToLower(args[0])]

		if chanExists {
			if channel.mode.noExternal {
				if _, inChannel := channel.clientMap[strings.ToLower(e.client.nick)]; !inChannel {
					//Not in channel, not allowed to send
					e.client.reply(errCannotSend, args[0])
					return
				}
			}
			if channel.mode.moderated {
				clientMode := channel.modeMap[strings.ToLower(e.client.nick)]
				if !clientMode.operator && !clientMode.voice {
					//It's moderated and we're not +v or +o, do nothing
					e.client.reply(errCannotSend, args[0])
					return
				}
			}
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

		channel, exists := s.channelMap[strings.ToLower(args[0])]
		if exists == false {
			e.client.reply(errNoSuchNick, args[0])
			return
		}

		channelName := args[0]

		if len(args) == 1 {
			e.client.reply(rplTopic, channelName, channel.topic)
			return
		}

		clientMode := channel.modeMap[strings.ToLower(e.client.nick)]
		if channel.mode.topicLocked && !clientMode.operator {
			e.client.reply(errNoPriv)
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
				if channel.mode.secret {
					if _, inChannel := channel.clientMap[strings.ToLower(e.client.nick)]; !inChannel {
						//Not in the channel, skip
						continue
					}
				}
				listItem := fmt.Sprintf("%s %d :%s", channelName, len(channel.clientMap), channel.topic)
				chanList = append(chanList, listItem)
			}

			e.client.reply(rplList, chanList...)

		} else {
			channels := strings.Split(args[0], ",")
			chanList := make([]string, 0, len(channels))

			for _, channelName := range channels {
				if channel, exists := s.channelMap[strings.ToLower(channelName)]; exists {
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

		if client, exists := s.clientMap[strings.ToLower(nick)]; exists {
			client.reply(rplKill, "An operator has disconnected you.")
			client.disconnect()
		} else {
			e.client.reply(errNoSuchNick, nick)
		}

	case command == "KICK":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 2 {
			e.client.reply(errMoreArgs)
			return
		}

		channelKey := strings.ToLower(args[0])
		targetKey := strings.ToLower(args[1])

		channel, channelExists := s.channelMap[channelKey]
		if !channelExists {
			e.client.reply(errNoSuchNick, args[0])
			return
		}

		target, targetExists := channel.clientMap[targetKey]
		if !targetExists {
			e.client.reply(errNoSuchNick, args[1])
			return
		}

		clientMode := channel.modeMap[e.client.key]
		if !clientMode.operator && !e.client.operator {
			e.client.reply(errNoPriv)
			return
		}

		reason := strings.Join(args[2:], " ")

		//It worked
		for _, client := range channel.clientMap {
			client.reply(rplKick, e.client.nick, channel.name, target.nick, reason)
		}

		delete(channel.clientMap, targetKey)
		delete(channel.modeMap, targetKey)
		delete(target.channelMap, channelKey)

	case command == "MODE":
		if e.client.registered == false {
			e.client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		channelKey := strings.ToLower(args[0])

		channel, channelExists := s.channelMap[channelKey]
		if !channelExists {
			e.client.reply(errNoSuchNick, args[0])
			return
		}
		mode := channel.mode

		if len(args) == 1 {
			//No more args, they just want the mode
			e.client.reply(rplChannelModeIs, args[0], mode.String(), "")
			return
		}

		if cm, ok := channel.modeMap[strings.ToLower(e.client.nick)]; !ok || !cm.operator {
			//Not a channel operator.

			//If they're not an irc operator either, they'll fail
			if !e.client.operator {
				e.client.reply(errNoPriv)
				return
			}
		}

		hasClient := false
		var oldClientMode, newClientMode *ClientMode
		var targetClient *Client
		if len(args) >= 3 {
			clientKey := strings.ToLower(args[2])
			oldClientMode, hasClient = channel.modeMap[clientKey]
			if hasClient {
				targetClient = channel.clientMap[clientKey]
				newClientMode = new(ClientMode)
				*newClientMode = *oldClientMode
			}
		}

		mod := strings.ToLower(args[1])
		if strings.HasPrefix(mod, "+") {
			for _, char := range mod {
				switch char {
				case 's':
					mode.secret = true
				case 't':
					mode.topicLocked = true
				case 'm':
					mode.moderated = true
				case 'n':
					mode.noExternal = true
				case 'o':
					if hasClient {
						newClientMode.operator = true
					}
				case 'v':
					if hasClient {
						newClientMode.voice = true
					}
				}
			}
		} else if strings.HasPrefix(mod, "-") {
			for _, char := range mod {
				switch char {
				case 's':
					mode.secret = false
				case 't':
					mode.topicLocked = false
				case 'm':
					mode.moderated = false
				case 'n':
					mode.noExternal = false
				case 'o':
					if hasClient {
						newClientMode.operator = false
					}
				case 'v':
					if hasClient {
						newClientMode.voice = false
					}
				}
			}
		}

		if hasClient {
			*oldClientMode = *newClientMode
		}
		channel.mode = mode

		for _, client := range channel.clientMap {
			if hasClient {
				client.reply(rplChannelModeIs, args[0], args[1], targetClient.nick)
			} else {
				client.reply(rplChannelModeIs, args[0], args[1], "")
			}
		}

	default:
		e.client.reply(errUnknownCommand, command)
	}
}
