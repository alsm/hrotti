package hrotti

import (
	"encoding/json"
	"os"
	"sync"
)

var pluginNodes map[string]Plugin
var pluginMutex sync.Mutex

//define the interface for a Plugin, any struct with these methods can be a plugin
type Plugin interface {
	Initialise() error
	AddSub(*Client, []string, byte, chan byte)
	DeleteSub(*Client, []string, chan bool)
}

func init() {
	//set up the plugin map if it's nil. This code should also be in an init() for
	//every plugin that's written as the init function for a plugin is where it
	//registers itself and we can't guarantee the order init functions are called in
	pluginMutex.Lock()
	if pluginNodes == nil {
		pluginNodes = make(map[string]Plugin)
	}
	pluginMutex.Unlock()
}

func StartPlugins() {
	pluginMutex.Lock()
	defer pluginMutex.Unlock()
	//plugins have already registered themselves as part of their init functions
	//so range on map and call Initialise() for each plugin.
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

//when a client disconnects and is cleansession true we want to remove all
//subscriptions that client held in all plugins.
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
