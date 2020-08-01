package samqttif

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	flogolog "github.com/project-flogo/core/support/log"
)

var espSupported bool

// (M)ap (S)tring (I)nterface
type MSI map[string]interface{}

type SensorValueMessage struct {
	Esp     bool
	Topic   string
	Payload MSI
}

func NewSensorValueMessage(entity string, datetime string) *SensorValueMessage {
	topic := ""
	if !espSupported {
		topic = "sdw" + entity
	} else {
		topic = "esp" + entity
	}

	svm := &SensorValueMessage{Esp: espSupported, Topic: topic}
	svm.Payload = make(MSI)
	if !svm.Esp {
		svm.Payload["entity"] = entity
		svm.Payload["commandId"] = uuid.New().String()
	} else {
		svm.Payload["messageId"] = uuid.New().String()
	}
	svm.Payload["datetime"] = datetime
	svm.Payload["values"] = make([]MSI, 0, 10)
	//svm["values"] = append(svm["values"], 5)
	return svm
}

func (svm *SensorValueMessage) AddValue(field string, amount float64) {
	value := MSI{"field": field, "amount": amount}
	// This is how you append to a slice of arbitrary maps
	svm.Payload["values"] = append(svm.Payload["values"].([]MSI), value)
}

func (svm *SensorValueMessage) AddValueAttrib(field string, amount float64, attributes MSI) {
	value := MSI{"field": field, "amount": amount, "attributes": attributes}
	// This is how you append to a slice of arbitrary maps
	svm.Payload["values"] = append(svm.Payload["values"].([]MSI), value)
}

func (svm *SensorValueMessage) AddValueAttribCreated(field string, amount float64, attributes MSI, created string) {
	var value MSI
	if svm.Esp {
		// ESP does not support created field
		value = MSI{"field": field, "amount": amount, "attributes": attributes}
	} else {
		value = MSI{"field": field, "amount": amount, "attributes": attributes, "created": created}
	}
	// This is how you append to a slice of arbitrary maps
	svm.Payload["values"] = append(svm.Payload["values"].([]MSI), value)
}

func (svm *SensorValueMessage) AddValueCreated(field string, amount float64, created string) {
	var value MSI
	if svm.Esp {
		// ESP does not support created field
		value = MSI{"field": field, "amount": amount}
	} else {
		value = MSI{"field": field, "amount": amount, "created": created}
	}
	// This is how you append to a slice of arbitrary maps
	svm.Payload["values"] = append(svm.Payload["values"].([]MSI), value)
}

func (svm *SensorValueMessage) json() []byte {

	jsonData, err := json.Marshal(svm.Payload)
	if err != nil {
		fmt.Println("Error: Marshal of sensor value message payload failed. Cause: ", err.Error())
	}
	return jsonData
}

// Activity is used to create a custom activity. Add values here to retain them.
// Objects used by the time are defined here.
// Common structure
type SAMqttClient struct {
	host               string
	port               string
	clientID           string
	client             mqtt.Client
	logger             flogolog.Logger
	report             []string
	connectedOnce      bool // Default false
	connectCallback    MqttCallback
	disconnectCallback MqttCallback
	//esp                bool
	debug bool // default true
}

type MqttCallback func()

func NewSAMqttClient(host string, port string, clientID string, esp bool, debug bool) *SAMqttClient {
	espSupported = esp
	client := &SAMqttClient{
		host:     host,
		port:     port,
		clientID: clientID,
		//esp:      esp,
		debug: debug}
	client.Initialize()
	return client
}

func (c *SAMqttClient) RegisterConnectionCallbacks(connect MqttCallback, disconnect MqttCallback) {
	c.connectCallback = connect
	c.disconnectCallback = disconnect
}

func (c *SAMqttClient) Initialize() {

	// onConnect defines the on connect handler which resets backoff variables.
	var onConnect mqtt.OnConnectHandler = func(client mqtt.Client) {
		fmt.Println("Client connected.")
		c.connectedOnce = true
		c.connectCallback()
	}

	// onDisconnect defines the connection lost handler for the mqtt client.
	var onDisconnect mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
		fmt.Println("Client disconnected. Error: ", err.Error())
		c.connectedOnce = false
		c.disconnectCallback()
	}

	if c.debug {
		mqtt.DEBUG = log.New(os.Stdout, "", 0)
		mqtt.ERROR = log.New(os.Stdout, "", 0)
	}

	opts := mqtt.NewClientOptions()

	broker := "tcp://" + c.host + ":" + c.port
	opts.AddBroker(broker)
	opts.SetClientID(c.clientID)
	//opts.SetConnectTimeout(25000 * time.Millisecond)
	opts.SetWriteTimeout(25 * time.Millisecond)
	opts.SetOnConnectHandler(onConnect)
	opts.SetConnectionLostHandler(onDisconnect)
	// Reconnect is used to recover connections without application intervention
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(5 * time.Second)

	// Create and connect a client using the above options.
	c.client = mqtt.NewClient(opts)
}

func linearContains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

func binaryContains(a []string, x string) bool {
	i := sort.Search(len(a), func(i int) bool { return x <= a[i] })
	if i < len(a) && a[i] == x {
		return true
	}
	return false
}

func (c *SAMqttClient) publishStatus(path string, status string, description string) {
	payload := make(map[string]string)
	payload["status"] = status
	payload["datetime"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	topicPrefix := ""
	if espSupported {
		payload["messageId"] = uuid.New().String()
		topicPrefix = "esp"
	} else {
		payload["commandId"] = uuid.New().String()
		topicPrefix = "sdw"
	}
	if len(description) > 0 {
		payload["description"] = description
	}
	// jsonData is a string
	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Println("Error marshal status message faild. Cause: ", err.Error())
		return
	}
	topic := topicPrefix + path
	c.Publish(topic, jsonData)
}

// PublishRunning publish RUNNING status for specified path
func (c *SAMqttClient) PublishRunning(path string) {
	c.publishStatus(path+"[status]", "RUNNING", "")
}

// PublishNotRunning publishes NOT_RUNNING status for specified path
func (c *SAMqttClient) PublishNotRunning(path string) {
	c.publishStatus(path+"[status]", "NOT_RUNNING", "")
}

// PublishError publishes ERROR status for specified path with description
func (c *SAMqttClient) PublishError(path string, description string) {
	c.publishStatus(path+"[status]", "ERROR", description)
}

// Cleanup was expected to be called when the application stops.
func (c *SAMqttClient) Cleanup() error {

	c.client.Disconnect(10)
	return nil
}

func (c *SAMqttClient) Connect() error {
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		fmt.Println("Error: Failed to connect client. Error: ", token.Error())
		return token.Error()
	}
	// Used to send running when connects. I don't know that is what
	// we want to do.
	return nil
}

func (c *SAMqttClient) PublishValueMessage(svm *SensorValueMessage) {
	c.Publish(svm.Topic+"[value]", svm.json())
}

// Publish is a wrapper for the publish call on the client object
func (c *SAMqttClient) Publish(topic string, payload []byte) error {

	if !c.client.IsConnected() && !c.connectedOnce {
		c.Connect()
	}
	if c.client.IsConnected() {
		if token := c.client.Publish(topic, 0, false, payload); token.Wait() && token.Error() != nil {
			fmt.Println("Failed to publish payload to device state topic")
			return token.Error()
		}
	}
	return nil
}
