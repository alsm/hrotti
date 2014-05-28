package hrotti

import (
	"io/ioutil"
	"log"
	"net/url"
)

//loggers
var (
	INFO     *log.Logger
	PROTOCOL *log.Logger
	ERROR    *log.Logger
	DEBUG    *log.Logger
)

//The default output for all the loggers is set to ioutil.Discard
func init() {
	INFO = log.New(ioutil.Discard, "", 0)
	PROTOCOL = log.New(ioutil.Discard, "", 0)
	ERROR = log.New(ioutil.Discard, "", 0)
	DEBUG = log.New(ioutil.Discard, "", 0)
}

//ListenerConfig is a struct containing a URL
type ListenerConfig struct {
	URL *url.URL
}

//NewListenerConfig returns a pointer to a ListenerConfig prepared to listen
//on the URL specified as rawURL
func NewListenerConfig(rawURL string) *ListenerConfig {
	listenerURL, err := url.Parse(rawURL)
	if err != nil {
		return nil
	}
	l := &ListenerConfig{URL: listenerURL}
	return l
}
