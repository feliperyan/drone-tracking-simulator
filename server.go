package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/feliperyan/drone-tracking-simulator/dronedeliverysimul"
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
	airportList      []dronedeliverysimul.AirportConfig
)

const airportConfigJSONString = `[{
	"name":"air1", 
	"NE":{"lat":-33.8073, "lon":151.1606},  
	"SW":{"lat":-33.8972, "lon":151.2738},
	"drones": 2,
	"minDel": 3,
	"maxDel":3
}]`

// AirportConfig holds simple config to include airports for drones

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

	eventLoopSeconds, _ = strconv.Atoi(getOSEnvOrReplacement("FRYAN_EVENT_LOOP_SECS", "1"))

	airporStringtList := getOSEnvOrReplacement("FRYAN_AIRPORTS", airportConfigJSONString)
	airportList = dronedeliverysimul.GetAirportConfigFromJSONString(airporStringtList)
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

func runAirport(imDone chan bool, stopMe chan bool, airConf dronedeliverysimul.AirportConfig, firehose *kafka.Writer) {
	air := dronedeliverysimul.InitDroneController(airConf.Drones, airConf.MinDel, airConf.MaxDel, airConf.NE, airConf.SW, 0.003, airConf.Name)
	ctx := context.Background()

	for {
		select {
		case <-stopMe:
			imDone <- true
			return
		default:
			messages := make([]kafka.Message, 0)
			air.TickUpdate()
			for i := range air.Drones {
				msg := kafka.Message{
					Key:   []byte(fmt.Sprintf("Airport-%s", airConf.Name)),
					Value: air.Drones[i].GetStringJSON(),
				}
				messages = append(messages, msg)
				fmt.Println("Drone msg: ", string(air.Drones[i].GetStringJSON()))
			}
			err := firehose.WriteMessages(ctx, messages...)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Println("Tick")
			time.Sleep(1 * time.Second)
		}
	}
}

func runAirportSingle(imDone chan bool, stopMe chan bool, airConf dronedeliverysimul.AirportConfig, firehose *kafka.Writer) {
	air := dronedeliverysimul.InitDroneController(airConf.Drones, airConf.MinDel, airConf.MaxDel, airConf.NE, airConf.SW, 0.003, airConf.Name)
	ctx := context.Background()

	for {
		select {
		case <-stopMe:
			imDone <- true
			return
		default:
			air.TickUpdate()
			for i := range air.Drones {
				msg := kafka.Message{
					Key:   []byte(fmt.Sprintf("Airport-%s", airConf.Name)),
					Value: air.Drones[i].GetStringJSON(),
				}
				fmt.Println("Drone msg: ", string(air.Drones[i].GetStringJSON()))
				err := firehose.WriteMessages(ctx, msg)
				if err != nil {
					fmt.Println(err)
				}
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

func main() {
	initVariables()

	fmt.Printf("Initialising producer. Broker: %s | topic: %s | airports: %v\n", theBroker, theTopic, airportList)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	w := initialiseKafkaProducer(usingTLS)

	fmt.Println("Number of Airport and Goroutines:", len(airportList))
	for _, air := range airportList {
		// go runAirport(allDone, stopGopher, air, w)
		go runAirportSingle(allDone, stopGopher, air, w)
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
