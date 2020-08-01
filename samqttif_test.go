package samqttif

import (
	"fmt"
	"os"
	"testing"
)

var testMode bool

func TestMain(m *testing.M) {
	testMode = true
	result := m.Run()
	os.Exit(result)
}

func TestInit(t *testing.T) {
	fmt.Println("testMode: ", testMode)
	client := NewSAMqttClient()
	fmt.Println(client)
	client.initialize()
	paths := []string{"/a/b/c", "/b/c/d"}
	client.SetEntitiesPaths(paths)
	fmt.Println(client)
	client.PublishAllRunning()
}
