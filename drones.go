package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

// GPSCoord a simple latitude and longitude type
type GPSCoord struct {
	Lat float64
	Lon float64
}

// Drone represents a Drone
type Drone struct {
	// Looks like 0.0003 is about 10 meters
	CurrentPosition GPSCoord
	Destinations    []GPSCoord
	NextDestination int
	Speed           float64
	Name            string
}

// DroneController keeps track of drone state and updates state
type DroneController struct {
	Drones           []*Drone
	NWBoundary       GPSCoord
	SEBoundary       GPSCoord
	Airport          GPSCoord
	ControllerPrefix string
	droneMinDrops    int
	droneMaxDrops    int
	droneSpeed       float64
}

func (p1 GPSCoord) equal(p2 GPSCoord) bool {
	if p1.Lat == p2.Lat && p1.Lon == p2.Lon {
		return true
	}
	return false
}

// InitDroneController return a * to a struct of same name.
// numOfDrones = how many drones to initialise
// min/max NumOfDrops = drone get allocated a random number of delivery points between min, max
// northWest/southEast Boundary will determine the area drones will fly in.
// speed = the distance per event loop that a drone covers, 0.0003 seems to equal about 10 meters in real word dist
func InitDroneController(numOfDrones, minNumOfDrops, maxNumOfDrops int, northWestBoundary, southEastBoundary GPSCoord, speed float64, prefix string) *DroneController {
	dc := &DroneController{NWBoundary: northWestBoundary, SEBoundary: southEastBoundary}
	dc.droneMinDrops = minNumOfDrops
	dc.droneMaxDrops = maxNumOfDrops
	dc.droneSpeed = speed
	dc.ControllerPrefix = prefix
	dc.Drones = make([]*Drone, numOfDrones)

	// set airport at the mid point of the boundary
	dc.Airport = getCentreOfBoundary(northWestBoundary, southEastBoundary)

	// init drones
	for i := 0; i < numOfDrones; i++ {
		d := initDroneRandom(dc.Airport, northWestBoundary, southEastBoundary, minNumOfDrops, maxNumOfDrops, speed)
		d.Name = fmt.Sprintf("%s-%v", prefix, i)
		dc.Drones[i] = d
	}

	return dc
}

func getCentreOfBoundary(nw, se GPSCoord) GPSCoord {
	p := &GPSCoord{}
	p.Lat = ((nw.Lat - se.Lat) / 2) + se.Lat
	p.Lon = ((nw.Lon - se.Lon) / 2) + se.Lon

	return *p
}

func initDroneRandom(startPos, northWestBoundary, southEastBoundary GPSCoord, minNumOfDrops, maxNumOfDrops int, speed float64) *Drone {
	dr := &Drone{}
	dr.Speed = speed
	dr.CurrentPosition = startPos
	dr.NextDestination = 0

	rand.Seed(time.Now().UnixNano())
	num := rand.Intn((maxNumOfDrops-minNumOfDrops)+1) + minNumOfDrops
	dr.Destinations = make([]GPSCoord, num+1)

	for i := 0; i < num; i++ {
		p := &GPSCoord{}
		p.Lat = ((northWestBoundary.Lat - southEastBoundary.Lat) * rand.Float64()) + southEastBoundary.Lat
		p.Lon = ((northWestBoundary.Lon - southEastBoundary.Lon) * rand.Float64()) + southEastBoundary.Lon

		dr.Destinations[i] = *p
	}

	// Last destination is back at the airport
	dr.Destinations[num] = startPos

	return dr
}

func distanceBetweenCoords(g1, g2 GPSCoord) float64 {
	sideA := g1.Lat - g2.Lat
	sideB := g1.Lon - g2.Lon
	hyp := math.Hypot(sideA, sideB)

	return hyp
}

func (d *Drone) getPositionTowardsDestination() GPSCoord {
	sideA := d.CurrentPosition.Lat - d.Destinations[d.NextDestination].Lat
	sideB := d.CurrentPosition.Lon - d.Destinations[d.NextDestination].Lon
	hyp := math.Hypot(sideA, sideB)

	// if we moved by speed we'd go further then the destination so "arrive".
	if d.Speed > hyp {
		return d.Destinations[d.NextDestination]
	}

	newPosLat := -1 * (d.Speed / (hyp / sideA))
	newPosLon := -1 * (d.Speed / (hyp / sideB))

	return GPSCoord{newPosLat, newPosLon}
}

// ratio is a number between 0 and 1. 0 returns A, 1 returns B.
func linearInterpolation(pointA, pointB GPSCoord, ratio float64) GPSCoord {
	a := GPSCoord{pointA.Lat * (1 - ratio), pointA.Lon * (1 - ratio)}
	b := GPSCoord{pointB.Lat * ratio, pointB.Lon * ratio}

	fmt.Println(a)
	fmt.Println(b)

	return GPSCoord{a.Lat + b.Lat, a.Lon + b.Lon}
}

// speed is the number of units to move
func trigo(a, b GPSCoord, speed float64) GPSCoord {
	sideA := a.Lat - b.Lat
	sideB := a.Lon - b.Lon
	hyp := math.Hypot(sideA, sideB)

	newPosLat := -1 * (speed / (hyp / sideA))
	newPosLon := -1 * (speed / (hyp / sideB))

	return GPSCoord{a.Lat + newPosLat, a.Lon + newPosLon}
}

// UpdatePositionTowardsDestination modifies the caller!
// Moves a step in "speed" distance towards next destination.
// If step would end after destination, we have arrived and destination
// is returned and NextDestination is updated.
func (d *Drone) UpdatePositionTowardsDestination() {
	newPos := d.getPositionTowardsDestination()

	// If the next position is our destination just use it.
	if newPos.Lat == d.Destinations[d.NextDestination].Lat &&
		newPos.Lon == d.Destinations[d.NextDestination].Lon {
		d.NextDestination++
		newLat := newPos.Lat
		newLon := newPos.Lon
		d.CurrentPosition = GPSCoord{newLat, newLon}
		return
	}

	newLat := d.CurrentPosition.Lat + newPos.Lat
	newLon := d.CurrentPosition.Lon + newPos.Lon

	d.CurrentPosition = GPSCoord{newLat, newLon}

}

// TickUpdate updates all drones that belong to the dc DroneController
func (dc *DroneController) TickUpdate() {
	for ite, dr := range dc.Drones {

		// first test if we have reached all our destinations
		if dr.CurrentPosition.equal(dc.Airport) && dr.NextDestination > 0 {
			dc.Drones[ite] = initDroneRandom(dc.Airport, dc.NWBoundary, dc.SEBoundary, dc.droneMinDrops, dc.droneMaxDrops, dc.droneSpeed)
		}

		dr.UpdatePositionTowardsDestination()
	}
}
