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
		operatorMap: make(map[string]string),
		motd:        "Welcome to IRC. Powered by Rosella."}
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
			log.Printf("Recovered from error when handling event: %+v", event)
			log.Println(err)
		}
	}(e)

	switch e.event {
	case connected:
		//Client connected
		e.client.reply(rplMOTD, s.motd)
	case disconnected:
		//Client disconnected
	case command:
		//Client send a command
		fields := strings.Fields(e.input)
		if len(fields) < 1 {
			return
		}

		if strings.HasPrefix(fields[0], ":") {
			fields = fields[1:]
		}
		command := strings.ToUpper(fields[0])
		args := fields[1:]

		s.handleCommand(e.client, command, args)
	}
}

func (s *Server) handleCommand(client *Client, command string, args []string) {

	switch command {
	case "PING":
		client.reply(rplPong)
	case "INFO":
		client.reply(rplInfo, "Rosella IRCD github.com/eXeC64/Rosella")
	case "VERSION":
		client.reply(rplVersion, VERSION)
	case "NICK":
		if len(args) < 1 {
			client.reply(errNoNick)
			return
		}

		newNick := args[0]

		//Check newNick is of valid formatting (regex)
		if nickRegexp.MatchString(newNick) == false {
			client.reply(errInvalidNick, newNick)
			return
		}

		if _, exists := s.clientMap[strings.ToLower(newNick)]; exists {
			client.reply(errNickInUse, newNick)
			return
		}

		//Protect the server name from being used
		if strings.ToLower(newNick) == strings.ToLower(s.name) {
			client.reply(errNickInUse, newNick)
			return
		}

		client.setNick(newNick)

	case "USER":
		if client.nick == "" {
			client.reply(rplKill, "Your nickname is already being used", "")
			client.disconnect()
		} else {
			client.reply(rplWelcome)
			client.registered = true
		}

	case "JOIN":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			client.reply(errMoreArgs)
			return
		}

		if args[0] == "0" {
			//Quit all channels
			for channel := range client.channelMap {
				client.partChannel(channel, "Disconnecting")
			}
			return
		}

		channels := strings.Split(args[0], ",")
		for _, channel := range channels {
			//Join the channel if it's valid
			if channelRegexp.Match([]byte(channel)) {
				client.joinChannel(channel)
			}
		}

	case "PART":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			client.reply(errMoreArgs)
			return
		}

		reason := strings.Join(args[1:], " ")

		channels := strings.Split(args[0], ",")
		for _, channel := range channels {
			//Part the channel if it's valid
			if channelRegexp.Match([]byte(channel)) {
				client.partChannel(channel, reason)
			}
		}

	case "PRIVMSG":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 2 {
			client.reply(errMoreArgs)
			return
		}

		message := strings.Join(args[1:], " ")

		channel, chanExists := s.channelMap[strings.ToLower(args[0])]
		client2, clientExists := s.clientMap[strings.ToLower(args[0])]

		if chanExists {
			if channel.mode.noExternal {
				if _, inChannel := channel.clientMap[strings.ToLower(client.nick)]; !inChannel {
					//Not in channel, not allowed to send
					client.reply(errCannotSend, args[0])
					return
				}
			}
			if channel.mode.moderated {
				clientMode := channel.modeMap[strings.ToLower(client.nick)]
				if !clientMode.operator && !clientMode.voice {
					//It's moderated and we're not +v or +o, do nothing
					client.reply(errCannotSend, args[0])
					return
				}
			}
			for _, c := range channel.clientMap {
				if c != client {
					c.reply(rplMsg, client.nick, args[0], message)
				}
			}
		} else if clientExists {
			client.reply(rplMsg, client.nick, client2.nick, message)
		} else {
			client.reply(errNoSuchNick, args[0])
		}

	case "QUIT":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		client.disconnect()

	case "TOPIC":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			client.reply(errMoreArgs)
			return
		}

		channel, exists := s.channelMap[strings.ToLower(args[0])]
		if exists == false {
			client.reply(errNoSuchNick, args[0])
			return
		}

		if len(args) == 1 {
			client.reply(rplTopic, channel.name, channel.topic)
			return
		}

		clientMode := channel.modeMap[strings.ToLower(client.nick)]
		if channel.mode.topicLocked && !clientMode.operator {
			client.reply(errNoPriv)
			return
		}

		if args[1] == ":" {
			channel.topic = ""
			for _, client := range channel.clientMap {
				client.reply(rplNoTopic, channel.name)
			}
		} else {
			topic := strings.Join(args[1:], " ")
			topic = strings.TrimPrefix(topic, ":")
			channel.topic = topic

			for _, client := range channel.clientMap {
				client.reply(rplTopic, channel.name, channel.topic)
			}
		}

	case "LIST":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) == 0 {
			chanList := make([]string, 0, len(s.channelMap))

			for channelName, channel := range s.channelMap {
				if channel.mode.secret {
					if _, inChannel := channel.clientMap[strings.ToLower(client.nick)]; !inChannel {
						//Not in the channel, skip
						continue
					}
				}
				listItem := fmt.Sprintf("%s %d :%s", channelName, len(channel.clientMap), channel.topic)
				chanList = append(chanList, listItem)
			}

			client.reply(rplList, chanList...)

		} else {
			channels := strings.Split(args[0], ",")
			chanList := make([]string, 0, len(channels))

			for _, channelName := range channels {
				if channel, exists := s.channelMap[strings.ToLower(channelName)]; exists {
					listItem := fmt.Sprintf("%s %d :%s", channelName, len(channel.clientMap), channel.topic)
					chanList = append(chanList, listItem)
				}
			}

			client.reply(rplList, chanList...)
		}
	case "OPER":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 2 {
			client.reply(errMoreArgs)
			return
		}

		username := args[0]
		password := args[1]

		if hashedPassword, exists := s.operatorMap[username]; exists {
			h := sha1.New()
			io.WriteString(h, password)
			pass := fmt.Sprintf("%x", h.Sum(nil))
			if hashedPassword == pass {
				client.operator = true
				client.reply(rplOper)
				return
			}
		}
		client.reply(errPassword)

	case "KILL":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if client.operator == false {
			client.reply(errNoPriv)
			return
		}

		if len(args) < 1 {
			client.reply(errMoreArgs)
			return
		}

		nick := args[0]

		reason := strings.Join(args[1:], " ")

		client, exists := s.clientMap[strings.ToLower(nick)]
		if !exists {
			client.reply(errNoSuchNick, nick)
			return
		}

		client.reply(rplKill, client.nick, reason)
		client.disconnect()

	case "KICK":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 2 {
			client.reply(errMoreArgs)
			return
		}

		channelKey := strings.ToLower(args[0])
		targetKey := strings.ToLower(args[1])

		channel, channelExists := s.channelMap[channelKey]
		if !channelExists {
			client.reply(errNoSuchNick, args[0])
			return
		}

		target, targetExists := channel.clientMap[targetKey]
		if !targetExists {
			client.reply(errNoSuchNick, args[1])
			return
		}

		clientMode := channel.modeMap[client.key]
		if !clientMode.operator && !client.operator {
			client.reply(errNoPriv)
			return
		}

		reason := strings.Join(args[2:], " ")

		//It worked
		for _, client := range channel.clientMap {
			client.reply(rplKick, client.nick, channel.name, target.nick, reason)
		}

		delete(channel.clientMap, targetKey)
		delete(channel.modeMap, targetKey)
		delete(target.channelMap, channelKey)

	case "MODE":
		if client.registered == false {
			client.reply(errNotReg)
			return
		}

		if len(args) < 1 {
			client.reply(errMoreArgs)
			return
		}

		channelKey := strings.ToLower(args[0])

		channel, channelExists := s.channelMap[channelKey]
		if !channelExists {
			client.reply(errNoSuchNick, args[0])
			return
		}
		mode := channel.mode

		if len(args) == 1 {
			//No more args, they just want the mode
			client.reply(rplChannelModeIs, args[0], mode.String(), "")
			return
		}

		if cm, ok := channel.modeMap[strings.ToLower(client.nick)]; !ok || !cm.operator {
			//Not a channel operator.

			//If they're not an irc operator either, they'll fail
			if !client.operator {
				client.reply(errNoPriv)
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
				client.reply(rplChannelModeIs, channel.name, args[1], targetClient.nick)
			} else {
				client.reply(rplChannelModeIs, channel.name, args[1], "")
			}
		}

	default:
		client.reply(errUnknownCommand, command)
	}
}
