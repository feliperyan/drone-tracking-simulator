package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/feliperyan/drone-tracking-simulator/dronedeliverysimul"
)

const airportConfigJSONString = `[{
	"name":"air1", 
	"NE":{"lat":-33.8073, "lon":151.1606},  
	"SW":{"lat":-33.8972, "lon":151.2738},
	"drones": 2,
	"minDel": 1,
	"maxDel":1
}]`

type clientMessage struct {
	Command string            `json:"commands"`
	Message map[string]string `json:"message"`
}

// TODO: New channel map string:string that receives something like air1-0:stop
// and passes that to TickUpdate() so it removes it keeps it in place instead of updating it.
// air1-0:go would resume movement.

func runAirport(
	imDone chan bool,
	stopMe chan bool,
	airConf dronedeliverysimul.AirportConfig,
	payload chan *dronedeliverysimul.Drone,
	eventLoopSeconds int,
	clientMessages <-chan string) {

	air := dronedeliverysimul.InitDroneController(airConf.Drones, airConf.MinDel, airConf.MaxDel, airConf.NE, airConf.SW, 0.003, airConf.Name)

	dronesStopped := make(map[string]string)

	for {
		select {
		case <-stopMe:
			imDone <- true
			return
		case clientMsg := <-clientMessages:
			m := clientMessage{}
			json.Unmarshal([]byte(clientMsg), &m)
			dr := m.Message["drone"]

			_, found := dronesStopped[dr]
			if found {
				fmt.Println("Resuming drone: ", dr)
				delete(dronesStopped, dr)
			} else {
				fmt.Println("Stopping drone: ", dr)
				dronesStopped[dr] = "stop"
			}
		default:
			air.TickUpdate(dronesStopped)

			for i := range air.Drones {
				// fmt.Println("Drone msg: ", string(air.Drones[i].GetStringJSON()))
				fmt.Println("Sending to websocket...")
				d := air.Drones[i]
				payload <- d
			}

			fmt.Println("Tick")
			time.Sleep(time.Duration(eventLoopSeconds) * time.Second)
		}
	}
}

func getOSEnvOrReplacement(envVarName, valueIfNotFound string) string {
	thing, found := os.LookupEnv(envVarName)
	if found {
		return thing
	}
	return valueIfNotFound
}

func runSimulationMain(payloadForWS chan *dronedeliverysimul.Drone, killSig <-chan os.Signal, clientMessages <-chan string) {

	eventLoopSeconds, _ := strconv.Atoi(getOSEnvOrReplacement("FRYAN_EVENT_LOOP_SECS", "1"))
	airporStringtList := getOSEnvOrReplacement("FRYAN_AIRPORTS", airportConfigJSONString)
	airportList := dronedeliverysimul.GetAirportConfigFromJSONString(airporStringtList)

	fmt.Printf("Initialising simulation: airports: %v\n", airportList)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	fmt.Println("Number of Airport and Goroutines:", len(airportList))
	for _, air := range airportList {
		go runAirport(allDone, stopGopher, air, payloadForWS, eventLoopSeconds, clientMessages)
	}

	finito := <-killSig
	fmt.Println("\nReceived signal: ", finito)
	fmt.Println("Exiting Routines.")
	for n := 0; n < len(airportList); n++ {
		stopGopher <- true
	}

	// await finish
	for n := 0; n < len(airportList); n++ {
		<-allDone
	}

	fmt.Println("All Simulation routines exited.")

}
