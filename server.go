package main

import (
	"fmt"
)

func main() {
	fmt.Println("Server starting")
	airport := InitDroneController(5, 5, 10, GPSCoord{10, 5}, GPSCoord{2, 20}, 1.0)

}
