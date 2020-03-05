package main

import (
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
	"drones": 1,
	"minDel": 1,
	"maxDel":1
}]`

// AirportConfig holds simple config to include airports for drones

func runAirport(imDone chan bool, stopMe chan bool, airConf dronedeliverysimul.AirportConfig, payload chan *dronedeliverysimul.Drone, eventLoopSeconds int) {
	air := dronedeliverysimul.InitDroneController(airConf.Drones, airConf.MinDel, airConf.MaxDel, airConf.NE, airConf.SW, 0.003, airConf.Name)

	for {
		select {
		case <-stopMe:
			imDone <- true
			return
		default:
			air.TickUpdate()

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

func runSimulationMain(payloadForWS chan *dronedeliverysimul.Drone, killSig chan os.Signal) {

	eventLoopSeconds, _ := strconv.Atoi(getOSEnvOrReplacement("FRYAN_EVENT_LOOP_SECS", "10"))
	airporStringtList := getOSEnvOrReplacement("FRYAN_AIRPORTS", airportConfigJSONString)
	airportList := dronedeliverysimul.GetAirportConfigFromJSONString(airporStringtList)

	fmt.Printf("Initialising simulation: airports: %v\n", airportList)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	fmt.Println("Number of Airport and Goroutines:", len(airportList))
	for _, air := range airportList {
		go runAirport(allDone, stopGopher, air, payloadForWS, eventLoopSeconds)
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
