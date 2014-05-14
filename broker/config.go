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

func init() {
	INFO = log.New(ioutil.Discard, "", 0)
	PROTOCOL = log.New(ioutil.Discard, "", 0)
	ERROR = log.New(ioutil.Discard, "", 0)
	DEBUG = log.New(ioutil.Discard, "", 0)
}

type ListenerConfig struct {
	URL  *url.URL `json:"url"`
	stop chan struct{}
}
