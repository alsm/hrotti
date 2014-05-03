package plugins

import (
	"strings"
	"sync"
)

type RedirectPlugin struct {
	sync.RWMutex
	Redirects  map[string]string
	fauxClient *Client
	stop       chan struct{}
}

func init() {
	pluginMutex.Lock()
	if pluginNodes == nil {
		pluginNodes = make(map[string]Plugin)
	}
	pluginNodes["$redirect"] = &RedirectPlugin{}
	pluginMutex.Unlock()
}

func (rp *RedirectPlugin) Initialise() error {
	rp.Redirects = make(map[string]string)
	if err := ReadPluginConfig("redirect_plugin_config.json", &rp.Redirects); err != nil {
		return err
	}
	rp.fauxClient = NewClient(nil, nil, "$redirectpluginclient")
	INFO.Println("Redirects:", rp.Redirects)
	for source, _ := range rp.Redirects {
		rp.fauxClient.AddSubscription([]string{source}, []byte{0})
	}
	go rp.Run()
	return nil
}

func (rp *RedirectPlugin) AddSub(client *Client, topic []string, qos byte, complete chan byte) {
	rp.Lock()
	defer rp.Unlock()
	sourceAndDest := strings.Split(strings.Join(topic[1:], ""), ",")
	rp.Redirects[sourceAndDest[0]] = sourceAndDest[1]
	rp.fauxClient.AddSubscription([]string{sourceAndDest[0]}, []byte{0})
	complete <- 0
}

func (rp *RedirectPlugin) DeleteSub(client *Client, topic []string, complete chan bool) {
	rp.Lock()
	defer rp.Unlock()
	if topic != nil {
		source := strings.Join(topic[1:], "")
		rp.fauxClient.RemoveSubscription(source)
		delete(rp.Redirects, source)
	}
	complete <- true
}

func (rp *RedirectPlugin) Run() {
	for {
		select {
		case <-rp.stop:
			return
		case msg := <-rp.fauxClient.outboundMessages:
			rp.RLock()
			if dest, ok := rp.Redirects[msg.topicName]; ok {
				rp.fauxClient.rootNode.DeliverMessage(strings.Split(dest, "/"), msg)
			}
			rp.RUnlock()
		}
	}
}
