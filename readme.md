[![travis-ci](https://travis-ci.org/feliperyan/drone-tracking-simulator.svg?branch=master)](https://travis-ci.org/feliperyan/drone-tracking-simulator)
[![Go Report Card](https://goreportcard.com/badge/github.com/feliperyan/drone-tracking-simulator)](https://goreportcard.com/report/github.com/feliperyan/drone-tracking-simulator)
# Drone Tracking Simulator

🚨🚨🚨This Heroku button uses the **paid** Kafka add-on 🚨🚨🚨

[![Deploy](https://www.herokucdn.com/deploy/button.svg)](https://heroku.com/deploy?template=https://github.com/feliperyan/drone-tracking-simulator)

## Purpose
This app simulates the tracking of a fleet of drones performing air-drop deliveries in a specified region. 

1. As the drones fly to their destinations they send location updates as GPS coordinates once per second.
2. Drones travel about 5 meters per second.
3. The "base" writes all the updates to a Kafka stream for processing.

## Usage
To deploy this app outside of Heroku please refer to the `initVariables()` function and provide the necessary env vars.

The main configuration to consider is the json string defined in the `FRYAN_AIRPORTS` env var. Each one of these JSON objects (notice it's an array) represents a _drone port_ where drones will take-off from. Drones will choose a number of delivery destinations at random between `MinDel` and `MaxDel` and the addresses for each delivery are completely random within the boundaries defined by the `NE` and `SW` coordinates.

## Explanation for each key:

1. `Name` = Every drone flying from this _droneport_ will be emit an event identifying itself as drone number N from airport X. So if `name="air1"` then drone2 is called `air1-2`.
2. `NE` = This is the _top left_ boundary of the location in the world of where the drones are allowed to fly, expressed as Lat and Lon. For example `-33.8561, 151.2153` roughly the Sydney Opera House.
3. `SW` = Conversely this is the _bottom right_ boundary. For example `33.8949, 151.2743` roughly Bondi Icebergs. Drones will fly within this _quadrant_ or square.
4. `Drones` = How many drones to fly and therefore track. The larger the number the more messages to Kafka, the more to process, the more load, etc.
5. `MinDel` = Minimum amount of deliveries each drone will perform, this adds some randomness to the simulation.
5. `MaxDel` = Maximum amount of deliveries each drone will perform, this adds some randomness to the simulation.

## Example

```
[{
    "name":"air1", 
    "NE":{"lat":-33.8073, "lon":151.1606},  
    "SW":{"lat":-33.8972, "lon":151.2738},
    "drones": 10,
    "minDel": 5,
    "maxDel":20
}]
```

## Kafka message example:
```
{"CurrentPosition":{"Lat":3,"Lon":3},"Destinations":[{"Lat":9,"Lon":5}],"NextDestination":0,"Speed":5,"Name":"drone-1"}
```

## To do:
1. Replace trigo implementation with linearInterpolation. 😀
2. Re-architecture so each dyno takes care of a different group of airports. 😕
3. Come up with a visualisation of what is going on. 😳
    * Check this repo for an update: https://github.com/feliperyan/react-redux-websocket-live-map
