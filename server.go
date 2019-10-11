package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/segmentio/kafka-go"
)

// Global variables:
var (
	theBroker        string
	allBrokers       []string
	theTopic         string
	usingTLS         bool
	certPEM          string
	keyPEM           string
	caPEM            string
	eventLoopSeconds int
	airportList      []AirportConfig
)

const airportConfigJSONString = `[{
	"name":"air1", 
	"NE":{"lat":-33.8073, "lon":151.1606},  
	"SW":{"lat":-33.8972, "lon":151.2738},
	"drones": 2000,
	"minDel": 5,
	"maxDel":5
},
{
	"name":"air2", 
	"NE":{"lat":-33.8073, "lon":151.1606},  
	"SW":{"lat":-33.8972, "lon":151.2738},
	"drones": 2000,
	"minDel": 5,
	"maxDel":5
}]`

// AirportConfig holds simple config to include airports for drones
type AirportConfig struct {
	Name   string
	NE     GPSCoord
	SW     GPSCoord
	Drones int
	MinDel int
	MaxDel int
}

func initVariables() {
	theBroker = getOSEnvOrReplacement("KAFKA_URL", "localhost:9092")
	_, usingTLS = os.LookupEnv("KAFKA_CLIENT_CERT")
	certPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT", "")
	keyPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT_KEY", "")
	caPEM = getOSEnvOrReplacement("KAFKA_TRUSTED_CERT", "")

	theBroker = strings.ReplaceAll(theBroker, "kafka+ssl://", "")
	allBrokers = strings.Split(theBroker, ",")

	theTopic = getOSEnvOrReplacement("FRYAN_TOPIC", "drone-coordinates")
	topicPrefix := getOSEnvOrReplacement("KAFKA_PREFIX", "")
	theTopic = fmt.Sprintf("%s%s", topicPrefix, theTopic)

	eventLoopSeconds, _ = strconv.Atoi(getOSEnvOrReplacement("FRYAN_EVENT_LOOP_SECS", "3"))

	airporStringtList := getOSEnvOrReplacement("FRYAN_AIRPORTS", airportConfigJSONString)
	airportList = getAirportConfigFromJSONString(airporStringtList)
}

func getAirportConfigFromJSONString(stringOfAirportConfig string) []AirportConfig {
	var manyAirports []AirportConfig
	err := json.Unmarshal([]byte(stringOfAirportConfig), &manyAirports)
	if err != nil {
		fmt.Println("*** ERROR unmarshalling! Error is: ", err)
	}

	return manyAirports
}

func initialiseKafkaProducer(needsTLS bool) *kafka.Writer {

	if !needsTLS {
		w := kafka.NewWriter(kafka.WriterConfig{
			Brokers:  allBrokers,
			Topic:    theTopic,
			Balancer: &kafka.LeastBytes{},
			Async:    true,
		})
		return w
	}

	tconf := getTLSConfig()
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
		TLS:       tconf,
	}

	w := kafka.NewWriter(kafka.WriterConfig{
		Brokers:  allBrokers,
		Topic:    theTopic,
		Balancer: &kafka.LeastBytes{},
		Dialer:   dialer,
	})

	return w
}

func getTLSConfig() *tls.Config {
	// Define TLS configuration
	certificate, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		panic(fmt.Sprintf("X509KeyPair errored out: %s", err))
	}

	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(caPEM)); !ok {
		panic("x509.NewCertPool errored out.")

	}

	return &tls.Config{
		Certificates:       []tls.Certificate{certificate},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
}

func runAirport(imDone chan bool, stopMe chan bool, airConf AirportConfig,
	currentTick chan int64, kafkaMessages chan []kafka.Message) {

	air := InitDroneController(airConf.Drones, airConf.MinDel, airConf.MaxDel, airConf.NE, airConf.SW, 0.003, airConf.Name)

	for {
		select {

		case <-stopMe:
			imDone <- true
			return

		case tickNum := <-currentTick:
			messages := make([]kafka.Message, 0)
			air.TickUpdate()
			for i := range air.Drones {
				d := air.Drones[i]
				d.Tick = tickNum
				msg := kafka.Message{
					Key:   []byte(fmt.Sprintf("Airport-%s", airConf.Name)),
					Value: d.getStringJSON(),
				}
				messages = append(messages, msg)
				//fmt.Println("Drone msg: ", string(air.Drones[i].getStringJSON()))
			}
			log.Printf("%s sending %d messages to KafkaMessages channel.\n", airConf.Name, len(messages))
			kafkaMessages <- messages
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

func main() {
	initVariables()
	var tick int64
	tick = 0
	tickChan := make(chan int64, len(airportList))
	kafkaMessagesToSend := make(chan []kafka.Message, len(airportList))
	ctx := context.Background()

	fmt.Printf("Initialising producer. Broker: %s | topic: %s | airports: %v\n", theBroker, theTopic, airportList)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	w := initialiseKafkaProducer(usingTLS)

	fmt.Println("Number of Airport and Goroutines:", len(airportList))

	// Start all airport simulations. Each will block on tickChan
	for _, air := range airportList {
		go runAirport(allDone, stopGopher, air, tickChan, kafkaMessagesToSend)
	}

outer:
	for {
		select {
		case finito := <-sigs: // kill signal
			fmt.Println("\nReceived signal: ", finito)
			fmt.Println("Exiting Routines.")
			for n := 0; n < len(airportList); n++ {
				stopGopher <- true
			}
			break outer

		case <-time.After(time.Duration(eventLoopSeconds) * time.Second):
			// Send the current tickNum to each gorountine, unblocking them.
			for range airportList {
				fmt.Println("Sending tick: ", tick)
				tickChan <- tick
			}

			// Block until we receive the slice of kafka messages from each goroutine
			for range airportList {
				newMessages := <-kafkaMessagesToSend
				start := time.Now()
				err := w.WriteMessages(ctx, newMessages...)
				if err != nil {
					log.Println("Error writing to kafka: ", err)
				}
				elapsed := time.Since(start)
				log.Printf("Writing to kafka took %s", elapsed)
			}
			// update tick of course.
			tick++
		}
	}

	fmt.Println("Awaiting for goroutines to finish...")
	// await finish
	for n := 0; n < len(airportList); n++ {
		<-allDone
	}

	fmt.Println("All routines exited. Finished.")

}
