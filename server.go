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

	"github.com/segmentio/kafka-go"
)

// Global variables:
var (
	theBroker  string
	allBrokers []string
	theTopic   string
	gophers    int
	usingTLS   bool
	certPEM    string
	keyPEM     string
	caPEM      string
)

func initVariables() {
	theBroker = getOSEnvOrReplacement("KAFKA_URL", "localhost:9092")
	theTopic = getOSEnvOrReplacement("FRYAN_TOPIC", "drone-coordinates")
	gophers, _ = strconv.Atoi(getOSEnvOrReplacement("FRYAN_GOPHERS", "1"))
	_, usingTLS = os.LookupEnv("KAFKA_CLIENT_CERT")
	certPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT", "")
	keyPEM = getOSEnvOrReplacement("KAFKA_CLIENT_CERT_KEY", "")
	caPEM = getOSEnvOrReplacement("KAFKA_TRUSTED_CERT", "")

	theBroker = strings.ReplaceAll(theBroker, "kafka+ssl://", "")
	allBrokers = strings.Split(theBroker, ",")

	topicPrefix := getOSEnvOrReplacement("KAFKA_PREFIX", "")
	theTopic = fmt.Sprintf("%s%s", topicPrefix, theTopic)

}

// depends on flag.Parse() or will use default values.
// TODO: add tls https://github.com/segmentio/kafka-go#tls-support
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
	fmt.Println(certPEM)
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

func runAirport(imDone chan bool, stopMe chan bool, myName string, firehose *kafka.Writer) {
	air := InitDroneController(1, 1, 1, GPSCoord{10, 2}, GPSCoord{3, 15}, 0.3, myName)
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
					Key:   []byte(fmt.Sprintf("Airport-%s", myName)),
					Value: []byte(fmt.Sprintf("%s", air.Drones[i].getStringJSON())),
				}
				err := firehose.WriteMessages(ctx, msg)
				if err != nil {
					fmt.Println(err)
				}
				fmt.Println(air.Drones[i].getStringJSON())
			}
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

	fmt.Printf("Initialising producer. Broker: %s | topic: %s | routines: %d\n", theBroker, theTopic, gophers)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	allDone := make(chan bool)
	stopGopher := make(chan bool)

	w := initialiseKafkaProducer(usingTLS)

	for i := 0; i < gophers; i++ {
		name := fmt.Sprintf("airport-%v", i)
		go runAirport(allDone, stopGopher, name, w)
	}

	finito := <-sigs
	fmt.Println("\nReceived signal: ", finito)
	fmt.Println("Exiting Routines.")
	for n := 0; n < gophers; n++ {
		stopGopher <- true
	}

	// await finish
	for n := 0; n < gophers; n++ {
		<-allDone
	}

	fmt.Println("All routines exited. Finished.")

}
