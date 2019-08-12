package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
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

// AirportConfig holds simple config to include airports for drones
type AirportConfig struct {
	Name string
	NE   GPSCoord
	SW   GPSCoord
}

func initVariables() {
	theBroker = getOSEnvOrReplacement("KAFKA_URL", "localhost:9092")
	_, usingTLS = os.LookupEnv("KAFKA_CLIENT_CERT")
	certPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT", "")
	keyPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT_KEY", "")
	caPEM = getOSEnvOrReplacement("KAFKA_TRUSTED_CERT", "")

	theBroker = strings.ReplaceAll(theBroker, "kafka+ssl://", "")
	allBrokers = strings.Split(theBroker, ",")

	topicPrefix := getOSEnvOrReplacement("KAFKA_PREFIX", "")
	theTopic = fmt.Sprintf("%s%s", topicPrefix, theTopic)

	theTopic = getOSEnvOrReplacement("FRYAN_TOPIC", "drone-coordinates")
	eventLoopSeconds, _ = strconv.Atoi(getOSEnvOrReplacement("FRYAN_EVENT_LOOP_SECS", "1"))

	airporStringtList := getOSEnvOrReplacement("FRYAN_AIRPORTS", `[{"name":"air1", {"lat":-33.8073, "lon":151.1606},  {"lat":-33.8972, "lon":151.2738}}]`)
	json.Unmarshal([]byte(airporStringtList), &airportList)
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

func runAirport(imDone chan bool, stopMe chan bool, airConf AirportConfig, firehose *kafka.Writer) {
	air := InitDroneController(1, 2, 2, airConf.NE, airConf.SW, 0.003, airConf.Name)
	ctx := context.Background()

	for {
		select {
		case <-stopMe:
			imDone <- true
			return
		default:
			for i := range air.Drones {
				air.TickUpdate()
				msg := kafka.Message{
					Key:   []byte(fmt.Sprintf("Airport-%s", airConf.Name)),
					Value: []byte(fmt.Sprintf("%s", air.Drones[i].getStringJSON())),
				}
				err := firehose.WriteMessages(ctx, msg)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println(air.Drones[i].getStringJSON())
			}
		}
		fmt.Println("Tick")
		time.Sleep(1 * time.Second)
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

	fmt.Printf("Initialising producer. Broker: %s | topic: %s | airports: %v\n", theBroker, theTopic, airportList)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	w := initialiseKafkaProducer(usingTLS)

	for _, air := range airportList {
		go runAirport(allDone, stopGopher, air, w)
	}

	finito := <-sigs
	fmt.Println("\nReceived signal: ", finito)
	fmt.Println("Exiting Routines.")
	for n := 0; n < len(airportList); n++ {
		stopGopher <- true
	}

	// await finish
	for n := 0; n < len(airportList); n++ {
		<-allDone
	}

	fmt.Println("All routines exited. Finished.")

}
