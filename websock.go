package main

// 1. Consume kafka messages
// 2. Parse format into json?
// 3. Broadcast to websocket clients
// 4. Allow client to determine what message group it wants?

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/feliperyan/drone-tracking-simulator/dronedeliverysimul"
	"github.com/gorilla/websocket"
)

var (
	upgrader    = websocket.Upgrader{}
	allMessages chan *dronedeliverysimul.Drone
	closeConn   chan *websocket.Conn
	allConns    chan *websocket.Conn
	addr        = flag.String("port", "8080", "http service address")

	msgFromClient chan string
)

func init() {
	allMessages = make(chan *dronedeliverysimul.Drone)
	closeConn = make(chan *websocket.Conn)
	allConns = make(chan *websocket.Conn)

	msgFromClient = make(chan string)
}

func indexWS(w http.ResponseWriter, r *http.Request) {
	content, err := ioutil.ReadFile("websocket.html")
	if err != nil {
		fmt.Println("Could not open file.", err)
	}
	fmt.Fprintf(w, "%s", content)
}

func incomingWebsocket(wri http.ResponseWriter, req *http.Request) {
	log.Print("incoming ws req origin: ", req.Header["Origin"])
	newConn, err := upgrader.Upgrade(wri, req, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer newConn.Close()

	allConns <- newConn

	for {
		mt, msg, err := newConn.ReadMessage()
		if err != nil {
			log.Println("Error: ", err)
			closeConn <- newConn
			return
		}
		if mt == websocket.TextMessage {
			fmt.Println("\nFROM CLIENT: ", string(msg))
			msgFromClient <- string(msg)
		}
	}
}

func processMessages(srv *http.Server, socks chan *websocket.Conn, messages chan *dronedeliverysimul.Drone, leaving chan *websocket.Conn, sigKill chan os.Signal) {
	allSocks := make([]*websocket.Conn, 0)

	for {
		select {

		case sig := <-sigKill: // server going down
			fmt.Println("\nKilling server: ", sig)
			for _, c := range allSocks {
				err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				if err != nil {
					log.Println("write: ", err)
				}
			}
			if err := srv.Shutdown(context.Background()); err != nil {
				log.Printf("HTTP server Shutdown Error: %v", err)
			}
			return

		case c := <-leaving: // client closed conn
			for i, v := range allSocks {
				if v == c { // remove client for slice
					allSocks[i] = allSocks[len(allSocks)-1]
					allSocks[len(allSocks)-1] = nil
					allSocks = allSocks[:len(allSocks)-1]
				}
			}
			fmt.Println("Client left. Total Clients: ", len(allSocks))

		case msg := <-messages: // broadcast message to clients
			fmt.Println("received: ", msg)
			for _, v := range allSocks {
				v.WriteJSON(msg)
			}

		case conn := <-socks: // new client connected, add to slice
			allSocks = append(allSocks, conn)
			fmt.Println("Client joined. Total Clients: ", len(allSocks))
		}
	}
}

func main() {

	flag.Parse()
	log.SetFlags(0)

	sigint := make(chan os.Signal, 1)
	sigIntProcessMessages := make(chan os.Signal, 1)
	sigIntSimulation := make(chan os.Signal, 1)
	signal.Notify(sigint, os.Interrupt)

	// Not secure at all so anyone can connect to this websocket...
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	theAddr := fmt.Sprintf(":%s", *addr)
	log.Println("Starting: ", theAddr)

	serv := &http.Server{Addr: theAddr, Handler: nil}

	// There's gotta be a better way than this.
	go func(killAll chan os.Signal, proc chan os.Signal, simul chan os.Signal) {
		finito := <-killAll
		proc <- finito
		simul <- finito
	}(sigint, sigIntProcessMessages, sigIntSimulation)

	go processMessages(serv, allConns, allMessages, closeConn, sigIntProcessMessages)
	go runSimulationMain(allMessages, sigIntSimulation, msgFromClient)

	http.HandleFunc("/", indexWS)
	http.HandleFunc("/ws", incomingWebsocket)
	log.Fatal(serv.ListenAndServe())

}
