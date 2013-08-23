package main

import (
	"crypto/tls"
	"flag"
	"log"
)

func main() {

	tlsKeyFile := flag.String("tls-key", "tls.key", "The private key file used for TLS")
	tlsCertFile := flag.String("tls-cert", "tls.crt", "The certificate file used for TLS")

	ircAddress := flag.String("irc-address", ":6697", "The address:port to bind to and listen for clients on")

	serverName := flag.String("irc-servername", "rosella", "Server name displayed to clients")

	flag.Parse()

	log.Printf("Rosella Initialising.")

	//Init rosella itself
	server, err := NewServer()
	if err != nil {
		panic(err)
	}

	server.name = *serverName
	server.Start()

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
