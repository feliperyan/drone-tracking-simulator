package dronedeliverysimul

import (
	"encoding/json"
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

	dummyMap := make(map[string]string)

	for i := 0; i <= distFloor; i++ {
		dc.TickUpdate(dummyMap)
		// fmt.Println(dc.Drones[0].getStringJSON())
		// fmt.Println("tick")
	}

	if !dc.Drones[0].CurrentPosition.equal(dc.Airport) {
		t.Error("Hasn't returned to destination: ", dc.Drones[0].CurrentPosition)
	}
}

func TestJSONRepresentation(t *testing.T) {
	d := getDrone()
	s := string(d.GetStringJSON())

	if strings.Compare(s,
		`{"CurrentPosition":{"Lat":3,"Lon":3},"Destinations":[{"Lat":9,"Lon":5}],"NextDestination":0,"Speed":5,"Name":"drone-1"}`) != 0 {
		t.Error("Not what we expected, got: ", s)
	}
}

func TestJSONUnmarshallGPSCoord(t *testing.T) {
	gpsString := `{"lat": -33.8073, "lon":151.1606}`
	var point GPSCoord

	json.Unmarshal([]byte(gpsString), &point)

	if point.Lat != -33.8073 || point.Lon != 151.1606 {
		t.Error("Expected lat -33.8073 and lon 151.1606 but got: ", point)
	}
}

func TestJSONUnmarshall(t *testing.T) {
	airportStringtList := `[{
		"name":"air1", 
		"NE":{"lat":-33.8073, "lon":151.1606},  
		"SW":{"lat":-33.8972, "lon":151.2738},
		"drones": 10,
		"minDel": 10,
		"maxDel":10
	}]`

	airs := GetAirportConfigFromJSONString(airportStringtList)

	if len(airs) == 0 {
		t.Error("Expected length of airports > 0. Got: ", len(airs))
		return
	}

	if airs[0].Name != "air1" {
		t.Error("Expected name of first airport to be air1. Got: ", airs[0].Name)
	}

	if airs[0].NE.Lat == 0 || airs[0].NE.Lon == 0 {
		t.Error("One of the GPSCoords had zero values. Got: ", airs[0].NE)
	}

	if airs[0].Drones != 10 || airs[0].MinDel != 10 || airs[0].MaxDel != 10 {
		t.Error("Expected 10 for Drones, Max and Min Deliveries. Got: ", airs[0])
	}

}

func BenchmarkTrigo(b *testing.B) {
	x := GPSCoord{10.0, 1.0}
	y := GPSCoord{1.0, 10.0}

	for i := 0; i < b.N; i++ {
		trigo(x, y, 0.003)
	}
}

func BenchmarkLinear(b *testing.B) {
	x := GPSCoord{10.0, 1.0}
	y := GPSCoord{1.0, 10.0}

	for i := 0; i < b.N; i++ {
		linearInterpolation(x, y, 0.000235702298569)
	}
}
