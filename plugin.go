package main

import (
	"encoding/json"
	"os"
	"sync"
)

var pluginNodes map[string]Plugin
var pluginMutex sync.Mutex

type Plugin interface {
	Initialise() error
	AddSub(*Client, []string, byte, chan bool)
	DeleteSub(*Client, []string, chan bool)
}

func init() {
	pluginMutex.Lock()
	if pluginNodes == nil {
		pluginNodes = make(map[string]Plugin)
	}
	pluginMutex.Unlock()
}

func StartPlugins() {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()
	for topic, plugin := range pluginNodes {
		err := plugin.Initialise()
		if err != nil {
			ERROR.Println("Failed to initialise plugin for", topic)
			delete(pluginNodes, topic)
		} else {
			INFO.Println("Initialised plugin for", topic)
		}
	}
}

func DeleteSubAllPlugins(client *Client) {
	complete := make(chan bool, 1)
	defer close(complete)
	for _, plugin := range pluginNodes {
		plugin.DeleteSub(client, nil, complete)
		<-complete
	}
}

func ReadPluginConfig(confFile string, result interface{}) error {
	file, err := os.Open(confFile)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)

	err = decoder.Decode(result)
	if err != nil {
		return err
	}
	return nil
}
