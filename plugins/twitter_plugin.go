package plugins

import (
	"errors"
	"sync"

	"github.com/darkhelmet/twitterstream"
)

type Secrets struct {
	ConsumerKey    string `json:"consumerKey,omitempty"`
	ConsumerSecret string `json:"consumerSecret,omitempty"`
	AccessToken    string `json:"accessToken,omitempty"`
	AccessSecret   string `json:"accessSecret,omitempty"`
}

type TwitterPlugin struct {
	sync.RWMutex
	client     *twitterstream.Client
	conn       *twitterstream.Connection
	config     *Secrets
	Subscribed map[*Client]byte
	stop       chan struct{}
	filter     string
}

func init() {
	pluginMutex.Lock()
	if pluginNodes == nil {
		pluginNodes = make(map[string]Plugin)
	}
	pluginNodes["$twitter"] = &TwitterPlugin{}
	pluginMutex.Unlock()
}

func (tp *TwitterPlugin) Initialise() error {
	if err := ReadPluginConfig("twitter_plugin_config.json", &tp.config); err != nil {
		return err
	}
	if tp.config.ConsumerKey == "" || tp.config.ConsumerSecret == "" || tp.config.AccessToken == "" || tp.config.AccessSecret == "" {
		return errors.New("Not all twitter secrets defined")
	}
	tp.Subscribed = make(map[*Client]byte)
	tp.client = twitterstream.NewClient(tp.config.ConsumerKey, tp.config.ConsumerSecret, tp.config.AccessToken, tp.config.AccessSecret)
	return nil
}

func (tp *TwitterPlugin) AddSub(client *Client, subscription []string, qos byte, complete chan byte) {
	tp.Lock()
	defer func() {
		complete <- 0
		tp.Subscribed[client] = qos
		tp.Unlock()
	}()
	var err error
	INFO.Println("Adding $twitter sub for", subscription[1], client.clientId)
	if subscription[1] != tp.filter {
		if tp.conn != nil {
			close(tp.stop)
			tp.conn.Close()
		}
		tp.stop = make(chan struct{})
		tp.conn, err = tp.client.Track(subscription[1])
		if err != nil {
			ERROR.Println(err.Error())
			return
		}
		tp.filter = subscription[1]
		go tp.Run()
	}
}

func (tp *TwitterPlugin) DeleteSub(client *Client, topic []string, complete chan bool) {
	tp.Lock()
	defer tp.Unlock()
	delete(tp.Subscribed, client)
	if len(tp.Subscribed) == 0 && tp.conn != nil {
		INFO.Println("All subscriptions gone, closing twitter connection")
		close(tp.stop)
		tp.conn.Close()
		tp.conn = nil
		tp.filter = ""
	}
	complete <- true
}

func (tp *TwitterPlugin) Run() {
	tweetChan := make(chan *twitterstream.Tweet, 2)
	go func() {
		for {
			tweet, err := tp.conn.Next()
			if err != nil {
				ERROR.Println("Twitter receive error")
				return
			}
			tweetChan <- tweet
			select {
			case <-tp.stop:
				return
			default:
			}
		}
	}()

	for {
		select {
		case tweet := <-tweetChan:
			tp.RLock()
			message := New(PUBLISH).(*publishPacket)
			message.Qos = 0
			message.topicName = "$twitter/" + tweet.User.ScreenName
			message.payload = []byte(tweet.Text)
			for client := range tp.Subscribed {
				select {
				case client.outboundMessages <- message:
				default:
				}
			}
			tp.RUnlock()
		case <-tp.stop:
			return
		}
	}
}
