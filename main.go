package main

import (
	"crypto/tls"
	"flag"
	"log"
	"os"
	"strings"
)

func main() {

	tlsKeyFile := flag.String("tls-key", "tls.key", "The private key file used for TLS")
	tlsCertFile := flag.String("tls-cert", "tls.crt", "The certificate file used for TLS")

	ircAddress := flag.String("irc-address", ":6697", "The address:port to bind to and listen for clients on")

	serverName := flag.String("irc-servername", "rosella", "Server name displayed to clients")

	authFile := flag.String("irc-authfile", "", "File containing usernames and passwords of operators.")

	flag.Parse()

	log.Printf("Rosella Initialising.")

	//Init rosella itself
	server := NewServer()
	server.name = *serverName

	if *authFile != "" {
		log.Printf("Loading auth file: %q", *authFile)

		f, err := os.Open(*authFile)
		if err != nil {
			log.Fatal(err)
		}
		data := make([]byte, 1024)
		size, err := f.Read(data)
		if err != nil {
			log.Fatal(err)
		}

		lines := strings.Split(string(data[:size]), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Fields(line)

			if len(fields) == 2 {
				server.operatorMap[fields[0]] = fields[1]
			}
		}
	}
	server.Run()

	tlsConfig := new(tls.Config)

	cert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
	if err != nil {
		log.Printf("Error loading tls certificate and key files.")
		log.Printf(err.Error())
		return
	}

	log.Printf("Loaded certificate and key successfully.")

	tlsConfig.Certificates = []tls.Certificate{cert}

	//Fills out tlsConfig.NameToCertificate
	tlsConfig.BuildNameToCertificate()

	tlsListener, err := tls.Listen("tcp", *ircAddress, tlsConfig)
	if err != nil {
		log.Printf("Could not open tls listener.")
		log.Printf(err.Error())
		return
	}

	log.Printf("Listening on %s", *ircAddress)

	for {
		conn, err := tlsListener.Accept()
		if err != nil {
			log.Printf("Error accepting connection.")
			log.Printf(err.Error())
			continue
		}

		server.HandleConnection(conn)
	}
}
