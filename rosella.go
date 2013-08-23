package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Server struct {
	signalChan chan int
	eventChan  chan Event
	running    bool

	name string

	clientMap  map[string]*Client  //Map of nicks → clients
	channelMap map[string]*Channel //Map of channel names → channels
}

type Client struct {
	server     *Server
	connection net.Conn
	signalChan chan int
	outputChan chan string
	nick       string
	channelMap map[string]*Channel
}

type Channel struct {
	name      string
	topic     string
	clientMap map[string]*Client
}

type Event struct {
	client *Client
	input  string
}

const (
	signalStop int = iota
	signalDisconnect
)

const (
	rplWelcome int = iota
	rplTopic
	rplNames
	rplNickChange
	errMoreArgs
	errNoNick
	errInvalidNick
	errNickInUse
	errAlreadyReg
	errNoSuchNick
)

var (
	nickRegexp    = regexp.MustCompile(`^[a-zA-Z\[\]_^{|}][a-zA-Z0-9\[\]_^{|}]*$`)
	channelRegexp = regexp.MustCompile(`^#[a-z0-9_\-]+$`)
)

func NewServer() (*Server, error) {
	return &Server{signalChan: make(chan int),
		eventChan:  make(chan Event),
		name:       "rosella",
		clientMap:  make(map[string]*Client),
		channelMap: make(map[string]*Channel)}, nil
}

func (s *Server) Start() {
	if s.running == false {
		s.running = true
		go s.serverThread()
	}
}

func (s *Server) HandleConnection(conn net.Conn) {

	client := &Client{server: s,
		connection: conn,
		outputChan: make(chan string),
		signalChan: make(chan int),
		channelMap: make(map[string]*Channel)}

	go client.clientThread()
}

func (s *Server) serverThread() {
	for {
		select {
		case s := <-s.signalChan:
			if s == signalStop {
				//TODO
				//Disconnect all clients then return
				return
			}
		case e := <-s.eventChan:
			s.handleEvent(e)
		}
	}
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
			e.client.reply(errInvalidNick)
			return
		}

		if _, exists := s.clientMap[newNick]; exists {
			e.client.reply(errNickInUse)
			return
		}

		e.client.setNick(newNick)

	case command == "USER":
		if e.client.nick == "" {
			//Give them a unique Guest nick
			newNick := fmt.Sprintf("Guest%d", rand.Int())
			for s.clientMap[newNick] != nil {
				newNick = fmt.Sprintf("Guest%d", rand.Int())
			}
		}
		e.client.reply(rplWelcome)

	case command == "JOIN":
		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		if args[0] == "0" {
			//Quit all channels
			for channel := range e.client.channelMap {
				s.partChannel(e.client, channel)
			}
			return
		}

		channels := strings.Split(args[0], ",")
		for _, channel := range channels {
			//Join the channel if it's valid
			if channelRegexp.Match([]byte(channel)) {
				s.joinChannel(e.client, channel)
			}
		}

	case command == "PART":
		if len(args) < 1 {
			e.client.reply(errMoreArgs)
			return
		}

		channels := strings.Split(args[0], ",")
		for _, channel := range channels {
			//Part the channel if it's valid
			if channelRegexp.Match([]byte(channel)) {
				s.partChannel(e.client, channel)
			}
		}

	case command == "PRIVMSG":
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
					c.outputChan <- fmt.Sprintf(":%s PRIVMSG %s %s", e.client.nick, args[0], message)
				}
			}
		} else if clientExists {
			client.outputChan <- fmt.Sprintf(":%s PRIVMSG %s %s", e.client.nick, client.nick, message)
		} else {
			e.client.reply(errNoSuchNick)
		}

	case command == "QUIT":
		//Stop the client, which will auto part channels and quit
		e.client.signalChan <- signalStop
	default:
		log.Printf("Unknown command: %q", fields[0])
	}
}

func (s *Server) joinChannel(client *Client, channelName string) {
	channel, exists := s.channelMap[channelName]
	if exists == false {
		channel = &Channel{name: channelName,
			topic:     "No Topic Set",
			clientMap: make(map[string]*Client)}
		s.channelMap[channelName] = channel
	}

	channel.clientMap[client.nick] = client
	client.channelMap[channelName] = channel

	for _, c := range channel.clientMap {
		c.outputChan <- fmt.Sprintf(":%s JOIN %s", client.nick, channelName)
	}

	//RPL_TOPIC
	client.outputChan <- fmt.Sprintf(":rosella 332 %s :%s", channelName, channel.topic)

	//RPL_NAMREPLY
	var nicks string
	for _, c := range channel.clientMap {
		nicks = fmt.Sprintf("%s %s", c.nick, nicks)
	}
	client.outputChan <- fmt.Sprintf(":rosella 353 %s = %s :%s", client.nick, channelName, nicks)
	//RPL_ENDOFNAMES
	client.outputChan <- ":rosella 366"
}

func (s *Server) partChannel(client *Client, channelName string) {
	channel, exists := s.channelMap[channelName]
	if exists == false {
		return
	}

	//Notify clients of the part
	for _, c := range channel.clientMap {
		c.outputChan <- fmt.Sprintf(":%s PART %s", client.nick, channelName)
	}

	delete(channel.clientMap, client.nick)
	delete(client.channelMap, channelName)
}

func (c *Client) clientThread() {
	defer c.connection.Close()

	readSignalChan := make(chan int, 1)
	writeSignalChan := make(chan int, 1)
	writeChan := make(chan string, 100)

	go c.readThread(readSignalChan)
	go c.writeThread(writeSignalChan, writeChan)

	for {
		select {
		case signal := <-c.signalChan:
			//Do stuff
			if signal == signalStop {
				readSignalChan <- signalStop
				writeSignalChan <- signalStop
				break
			}
		case line := <-c.outputChan:
			select {
			case writeChan <- line:
				//It worked
			default:
				log.Printf("Dropped a line for client: %q", c.nick)
				//Do nothing, dropping the line
			}
		}
	}

	//Part from all channels
	for channelName := range c.channelMap {
		c.server.partChannel(c, channelName)
	}

	//Remove from client list
	delete(c.server.clientMap, c.nick)
}

func (c *Client) readThread(signalChan chan int) {
	for {
		select {
		case signal := <-signalChan:
			if signal == signalStop {
				return
			}
		default:
			c.connection.SetReadDeadline(time.Now().Add(time.Second * 3))
			buf := make([]byte, 512)
			ln, err := c.connection.Read(buf)
			if err != nil {
				if err == io.EOF {
					//They must have dc'd
					c.signalChan <- signalStop
					return
				}
				continue
			}

			rawLines := buf[:ln]
			lines := bytes.Split(rawLines, []byte("\r\n"))
			for _, line := range lines {
				if len(line) > 0 {
					c.server.eventChan <- Event{client: c, input: string(line)}
				}
			}
		}
	}
}

func (c *Client) writeThread(signalChan chan int, outputChan chan string) {
	for {
		select {
		case signal := <-signalChan:
			if signal == signalStop {
				return
			}
		case output := <-outputChan:
			line := []byte(fmt.Sprintf("%s\r\n", output))

			c.connection.SetWriteDeadline(time.Now().Add(time.Second * 30))
			_, err := c.connection.Write(line)
			if err != nil {
				log.Printf("Write err: %q", err.Error())
				c.signalChan <- signalStop
				return
			}
		}
	}
}

//Send a reply to a user with the code specified
func (c *Client) reply(code int, args ...string) {
	switch code {
	case rplWelcome:
		c.outputChan <- fmt.Sprintf(":%s 001 %s :Welcome to %s", c.server.name, c.nick, c.server.name)
	case rplTopic:
		//Arg0 is channel, all following args are topic
		c.outputChan <- fmt.Sprintf(":%s 332 %s :%s", args[0], strings.Join(args[1:], " "))
	case rplNames:
		//Send the names
	case rplNickChange:
		c.outputChan <- fmt.Sprintf(":%s NICK %s", args[0], args[1])
	case errMoreArgs:
		c.outputChan <- fmt.Sprintf(":%s 461 :Not enough params", c.server.name)
	case errNoNick:
		c.outputChan <- fmt.Sprintf(":%s 431 :No nickanme given", c.server.name)
	case errInvalidNick:
		c.outputChan <- fmt.Sprintf(":%s 432 :Erronenous nickname", c.server.name)
	case errNickInUse:
		c.outputChan <- fmt.Sprintf(":%s 433 :Nick already in use", c.server.name)
	case errAlreadyReg:
		c.outputChan <- fmt.Sprintf(":%s 462 :You need a valid nick first", c.server.name)
	case errNoSuchNick:
		c.outputChan <- fmt.Sprintf("%s 401 :No such nick/channel", c.server.name)
	default:
		log.Printf("Client.reply() with unknown reply code: %d", code)
	}
}

func (c *Client) setNick(nick string) {
	if c.nick != "" {
		delete(c.server.clientMap, c.nick)
		for _, channel := range c.channelMap {
			delete(channel.clientMap, c.nick)
		}
	}

	//Set up new nick
	oldNick := c.nick
	c.nick = nick
	c.server.clientMap[c.nick] = c

	clients := make([]string, 0, 100)

	for _, channel := range c.channelMap {
		channel.clientMap[c.nick] = c

		//Collect list of client nicks who can see us
		for client := range channel.clientMap {
			clients = append(clients, client)
		}
	}

	//By sorting the nicks and skipping duplicates we send each client one message
	sort.Strings(clients)
	prevNick := ""
	for _, nick := range clients {
		if nick == prevNick {
			continue
		}
		prevNick = nick

		client, exists := c.server.clientMap[nick]
		if exists {
			client.reply(rplNickChange, oldNick, c.nick)
		}
	}
}
