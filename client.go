package main

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
)

func (c *Client) setNick(nick string) {
	//Set up new nick
	oldNick := c.nick
	oldKey := c.key
	c.nick = nick
	c.key = strings.ToLower(c.nick)

	delete(c.server.clientMap, oldKey)
	c.server.clientMap[c.key] = c

	//Update the relevant channels and notify everyone who can see us about our
	//nick change
	c.reply(rplNickChange, oldNick, c.nick)
	visited := make(map[*Client]struct{}, 100)
	for _, channel := range c.channelMap {
		delete(channel.clientMap, oldKey)

		for _, client := range channel.clientMap {
			if _, skip := visited[client]; skip {
				continue
			}
			client.reply(rplNickChange, oldNick, c.nick)
			visited[client] = struct{}{}
		}

		//Insert the new nick after iterating through channel.clientMap to avoid
		//sending a duplicate message to ourselves
		channel.clientMap[c.key] = c

		channel.modeMap[c.key] = channel.modeMap[oldKey]
		delete(channel.modeMap, oldKey)
	}
}

func (c *Client) joinChannel(channelName string) {
	newChannel := false

	channelKey := strings.ToLower(channelName)
	channel, exists := c.server.channelMap[channelKey]
	if exists == false {
		mode := ChannelMode{secret: true,
			topicLocked: true,
			noExternal:  true}
		channel = &Channel{name: channelName,
			topic:     "",
			clientMap: make(map[string]*Client),
			modeMap:   make(map[string]*ClientMode),
			mode:      mode}
		c.server.channelMap[channelKey] = channel
		newChannel = true
	}

	if _, inChannel := channel.clientMap[c.nick]; inChannel {
		//Client is already in the channel, do nothing
		return
	}

	mode := new(ClientMode)
	if newChannel {
		//If they created the channel, make them op
		mode.operator = true
	}

	channel.clientMap[c.nick] = c
	channel.modeMap[c.nick] = mode
	c.channelMap[channelKey] = channel

	for _, client := range channel.clientMap {
		client.reply(rplJoin, c.nick, channel.name)
	}

	if channel.topic != "" {
		c.reply(rplTopic, channel.name, channel.topic)
	} else {
		c.reply(rplNoTopic, channel.name)
	}

	//The capacity sets the max number of nicks to send per message
	nicks := make([]string, 0, 128)

	for _, client := range channel.clientMap {
		prefix := ""

		if mode, exists := channel.modeMap[client.nick]; exists {
			prefix = mode.Prefix()
		}

		if len(nicks) >= cap(nicks) {
			c.reply(rplNames, channelName, strings.Join(nicks, " "))
			nicks = nicks[:0]
		}

		nicks = append(nicks, fmt.Sprintf("%s%s", prefix, client.nick))
	}

	if len(nicks) > 0 {
		c.reply(rplNames, channelName, strings.Join(nicks, " "))
	}

	c.reply(rplEndOfNames, channelName)
}

func (c *Client) partChannel(channelName, reason string) {
	channelKey := strings.ToLower(channelName)
	channel, exists := c.server.channelMap[channelKey]
	if exists == false {
		return
	}

	if _, inChannel := channel.clientMap[c.key]; inChannel == false {
		//Client isn't in this channel, do nothing
		return
	}

	//Notify clients of the part
	for _, client := range channel.clientMap {
		client.reply(rplPart, c.nick, channel.name, reason)
	}

	delete(c.channelMap, channelKey)
	delete(channel.modeMap, c.key)
	delete(channel.clientMap, c.key)

	if len(channel.clientMap) == 0 {
		delete(c.server.channelMap, channelKey)
	}
}

//Send a reply to a user with the code specified
func (c *Client) reply(code replyCode, args ...string) {
	if c.connected() == false {
		return
	}

	switch code {
	case rplWelcome:
		c.send(":%s 001 %s :Welcome to %s", c.server.name, c.nick, c.server.name)
	case rplJoin:
		c.send(":%s JOIN %s", args[0], args[1])
	case rplPart:
		c.send(":%s PART %s %s", args[0], args[1], args[2])
	case rplTopic:
		c.send(":%s 332 %s %s :%s", c.server.name, c.nick, args[0], args[1])
	case rplNoTopic:
		c.send(":%s 331 %s %s :No topic is set", c.server.name, c.nick, args[0])
	case rplNames:
		c.send(":%s 353 %s = %s :%s", c.server.name, c.nick, args[0], args[1])
	case rplEndOfNames:
		c.send(":%s 366 %s %s :End of NAMES list", c.server.name, c.nick, args[0])
	case rplNickChange:
		c.send(":%s NICK %s", args[0], args[1])
	case rplKill:
		c.send(":%s KILL %s A %s", args[0], c.nick, args[1])
	case rplMsg:
		c.send(":%s PRIVMSG %s %s", args[0], args[1], args[2])
	case rplList:
		for _, listItem := range args {
			c.send(":%s 322 %s %s", c.server.name, c.nick, listItem)
		}
		c.send(":%s 323 %s", c.server.name, c.nick)
	case rplOper:
		c.send(":%s 381 %s :You are now an operator", c.server.name, c.nick)
	case rplChannelModeIs:
		c.send(":%s 324 %s %s %s %s", c.server.name, c.nick, args[0], args[1], args[2])
	case rplKick:
		c.send(":%s KICK %s %s %s", args[0], args[1], args[2], args[3])
	case rplInfo:
		c.send(":%s 371 %s :%s", c.server.name, c.nick, args[0])
	case rplVersion:
		c.send(":%s 351 %s %s", c.server.name, c.nick, args[0])
	case rplMOTD:
		motd := args[0]
		c.send(":%s 375 %s :- Message of the day - ", c.server.name, c.nick)
		for size := len(motd); size > 0; size = len(motd) {
			if size <= 80 {
				c.send(":%s 372 %s :- %s", c.server.name, c.nick, motd)
				break
			}
			c.send(":%s 372 %s :- %s", c.server.name, c.nick, motd[:80])
			motd = motd[80:]
		}
		c.send(":%s 376 %s :End of MOTD Command", c.server.name, c.nick)
	case rplPong:
		c.send(":%s PONG %s %s", c.server.name, c.nick, c.server.name)
	case errMoreArgs:
		c.send(":%s 461 %s :Not enough params", c.server.name, c.nick)
	case errNoNick:
		c.send(":%s 431 %s :No nickname given", c.server.name, c.nick)
	case errInvalidNick:
		c.send(":%s 432 %s %s :Erronenous nickname", c.server.name, c.nick, args[0])
	case errNickInUse:
		c.send(":%s 433 %s %s :Nick already in use", c.server.name, c.nick, args[0])
	case errAlreadyReg:
		c.send(":%s 462 :You need a valid nick first", c.server.name)
	case errNoSuchNick:
		c.send(":%s 401 %s %s :No such nick/channel", c.server.name, c.nick, args[0])
	case errUnknownCommand:
		c.send(":%s 421 %s %s :Unknown command", c.server.name, c.nick, args[0])
	case errNotReg:
		c.send(":%s 451 :You have not registered", c.server.name)
	case errPassword:
		c.send(":%s 464 %s :Error, password incorrect", c.server.name, c.nick)
	case errNoPriv:
		c.send(":%s 481 %s :Permission denied", c.server.name, c.nick)
	case errCannotSend:
		c.send(":%s 404 %s %s :Cannot send to channel", c.server.name, c.nick, args[0])
	}
}

func (c *Client) connected() bool {
	select {
	case <-c.stopChan:
		return false
	default:
		return true
	}
}

func (c *Client) disconnect() {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.connected() == true {
		close(c.stopChan)
	}
}

func (c *Client) send(format string, args ...interface{}) {
	select {
	case <-c.stopChan:
	case c.outputChan <- fmt.Sprintf(format, args...):
	}
}

func (c *Client) clientThread() {
	writeChan := make(chan string, 100)

	c.server.eventChan <- Event{client: c, event: connected}

	go c.readThread()
	go c.writeThread(writeChan)

	defer func() {
		//Part from all channels
		for channelName := range c.channelMap {
			c.partChannel(channelName, "Disconnecting")
		}

		delete(c.server.clientMap, c.key)

		c.connection.Close()
	}()

	for {
		select {
		case <-c.stopChan:
			return
		case line := <-c.outputChan:
			select {
			case writeChan <- line:
				continue
			default:
				c.disconnect()
			}
		}
	}

}

func (c *Client) readThread() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
			c.connection.SetReadDeadline(time.Now().Add(time.Second * 3))
			buf := make([]byte, 512)
			ln, err := c.connection.Read(buf)
			if err != nil {
				if err == io.EOF {
					c.disconnect()
					return
				}
				continue
			}

			rawLines := buf[:ln]
			rawLines = bytes.Replace(rawLines, []byte("\r\n"), []byte("\n"), -1)
			rawLines = bytes.Replace(rawLines, []byte("\r"), []byte("\n"), -1)
			lines := bytes.Split(rawLines, []byte("\n"))
			for _, line := range lines {
				if len(line) > 0 {
					c.server.eventChan <- Event{client: c, event: command, input: string(line)}
				}
			}
		}
	}
}

func (c *Client) writeThread(outputChan chan string) {
	for {
		select {
		case <-c.stopChan:
			return
		case output := <-outputChan:
			line := []byte(fmt.Sprintf("%s\r\n", output))

			c.connection.SetWriteDeadline(time.Now().Add(time.Second * 30))
			if _, err := c.connection.Write(line); err != nil {
				c.disconnect()
				return
			}
		}
	}
}
