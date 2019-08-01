package main

import (
	"fmt"
	"math"
	"strings"
	"testing"
)

func getDrone() *Drone {
	d := &Drone{}
	d.CurrentPosition = GPSCoord{3, 3}
	d.Destinations = make([]GPSCoord, 1)
	d.Destinations[0] = GPSCoord{9, 5}
	d.NextDestination = 0
	d.Speed = 5
	d.Name = "drone-1"

	return d
}

func getTravellingBackwardsDrone() *Drone {
	d := &Drone{}
	d.CurrentPosition = GPSCoord{9, 5}
	d.Destinations = make([]GPSCoord, 1)
	d.Destinations[0] = GPSCoord{3, 3}
	d.NextDestination = 0
	d.Speed = 5
	d.Name = "drone-1"

	return d
}

func getFarTravellingDrone() *Drone {
	d := &Drone{}
	d.CurrentPosition = GPSCoord{3, 3}
	d.Destinations = make([]GPSCoord, 1)
	d.Destinations[0] = GPSCoord{220, 190}
	d.NextDestination = 0
	d.Speed = 5
	d.Name = "drone-2"

	return d
}

// TestPrint is the first test
func TestPrint(t *testing.T) {
	d := getDrone()
	d.Name = "d-1"

	if d.Name != "d-1" {
		t.Error("Got wrong name", d.Name)
	}

}

//TestLocationOnAxis whether my Trigonometry is accurate
func TestLocationOnAxis(t *testing.T) {
	d := getDrone()
	g := d.getPositionTowardsDestination()

	if g.Lat != 4.743416490252569 || g.Lon != 1.5811388300841895 {
		t.Error("Values different from expectation: ", g)
	}
}

func TestUpdatePosition(t *testing.T) {
	d := getDrone()
	d.UpdatePositionTowardsDestination()

	if d.CurrentPosition.Lat != 7.743416490252569 || d.CurrentPosition.Lon != 4.5811388300841895 {
		t.Error("Values different from expectation: ", d.CurrentPosition)
	}
}

func TestArriveAtDestination(t *testing.T) {
	d := getDrone()
	d.UpdatePositionTowardsDestination()
	d.UpdatePositionTowardsDestination()

	if d.CurrentPosition.Lat != 9 || d.CurrentPosition.Lon != 5 {
		t.Error("Values different from expectation: ", d.CurrentPosition)
	}
}

func TestBackwardsArriveAtDestination(t *testing.T) {
	d := getTravellingBackwardsDrone()
	d.UpdatePositionTowardsDestination()
	d.UpdatePositionTowardsDestination()

	if d.CurrentPosition.Lat != 3 || d.CurrentPosition.Lon != 3 {
		t.Error("Values different from expectation: ", d.CurrentPosition)
	}
}

func TestArriveTravelFar(t *testing.T) {
	d := getFarTravellingDrone()
	for i := 0; i < 58; i++ {
		d.UpdatePositionTowardsDestination()
		// fmt.Println("Current Pos: ", d.CurrentPosition)
	}

	if d.CurrentPosition.Lat != 220 || d.CurrentPosition.Lon != 190 {
		t.Error("Values different from expectation: ", d.CurrentPosition)
	}
}

func TestInitDroneControllerHowManyDrones(t *testing.T) {
	nw := GPSCoord{10.0, 20.0}
	se := GPSCoord{5.0, -12.0}

	dc := InitDroneController(5, 5, 10, nw, se, 1.0, "Airport-A")

	if len(dc.Drones) != 5 {
		t.Error("Too many or too few Drones: ", len(dc.Drones))
	}
}

func TestInitRandomDroneBackToOrigin(t *testing.T) {
	nw := GPSCoord{10.0, 20.0}
	se := GPSCoord{2.0, -12.0}
	airport := GPSCoord{5.0, 3.0}

	d := initDroneRandom(airport, nw, se, 5, 10, 1)

	lastDest := len(d.Destinations) - 1

	if !airport.equal(d.Destinations[lastDest]) {
		t.Error("Last destination isn't origin. It is: ", d.Destinations[len(d.Destinations)-1])
	}

}

func TestTick(t *testing.T) {
	nw := GPSCoord{10.0, 4.0}
	se := GPSCoord{7.0, 13.0}
	dc := InitDroneController(1, 1, 1, nw, se, 1.0, "airport-A")

	// fmt.Printf("Airport: %v | borders: %v %v \n", dc.Airport, dc.NWBoundary, dc.SEBoundary)
	// fmt.Println("Drone: ", dc.Drones[0])

	dist := distanceBetweenCoords(dc.Drones[0].CurrentPosition, dc.Drones[0].Destinations[0])
	//fmt.Println("Distance: ", dist)

	// fmt.Println("Travelling")
	distFloor := int(math.Floor(dist))
	distFloor = (distFloor * 2) + 1 // get back to origin

	for i := 0; i <= distFloor; i++ {
		dc.TickUpdate()
		fmt.Println(dc.Drones[0].getStringJSON())
		fmt.Println("tick")
	}

	if !dc.Drones[0].CurrentPosition.equal(dc.Airport) {
		t.Error("Hasn't returned to destination: ", dc.Drones[0].CurrentPosition)
	}
}

func TestJSONRepresentation(t *testing.T) {
	d := getDrone()
	s := d.getStringJSON()

	if strings.Compare(s, "{drone: drone-1, lat: 3, lon: 3, dest:{lat:9, lon:5, num: 0}}") != 0 {
		t.Error("Not what we expected, got: ", s)
	}
}
