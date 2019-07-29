package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/segmentio/kafka-go"
)

var (
	theBroker = flag.String("broker", "localhost:9092", "kafka broker address")
	theTopic  = flag.String("topic", "drone-coordinates", "kafka topic")
	gophers   = flag.Int("routines", 3, "number of goroutines running airports")
)

// depends on flag.Parse() or will use default values.
func initialiseKafkaProducer() *kafka.Writer {
	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  []string{*theBroker},
		Topic:    *theTopic,
		Balancer: &kafka.LeastBytes{},
	})

	return w
}

func runAirport(imDone chan bool, stopMe chan bool, myName string, firehose *kafka.Writer) {
	air := InitDroneController(5, 5, 10, GPSCoord{10, 2}, GPSCoord{3, 15}, 0.3, myName)
	ctx := context.Background()

outerLoop:
	for {
		select {
		case <-stopMe:
			break outerLoop
		default:
			air.TickUpdate()
			for _, d := range air.Drones {
				msg := kafka.Message{
					Key:   []byte(fmt.Sprintf("Airport-%s", myName)),
					Value: []byte(fmt.Sprintf("%s", d.getStringJSON())),
				}
				err := firehose.WriteMessages(ctx, msg)
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	}

	imDone <- true
	return
}

func main() {
	flag.Parse()
	fmt.Printf("Initialising producer. Broker: %s | topic: %s | routines: %d\n", *theBroker, *theTopic, *gophers)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	w := initialiseKafkaProducer()

	for i := 0; i < *gophers; i++ {
		name := fmt.Sprintf("airport-%v", i)
		go runAirport(allDone, stopGopher, name, w)
	}

	finito := <-sigs
	fmt.Println("Received signal: ", finito)
	fmt.Println("Exiting Routines.")
	for n := 0; n < *gophers; n++ {
		stopGopher <- true
	}

	// await finish
	for n := 0; n < *gophers; n++ {
		<-allDone
	}

	fmt.Println("All routines exited. Finished.")

}
