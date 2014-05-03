package hrotti

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"os"
)

//loggers and the global inbound/outbound persistence. Persistence is an interface
var (
	INFO     *log.Logger
	PROTOCOL *log.Logger
	ERROR    *log.Logger
	DEBUG    *log.Logger
)

var logTargets map[string]io.Writer = map[string]io.Writer{
	"stdout":  os.Stdout,
	"stderr":  os.Stderr,
	"discard": ioutil.Discard,
}

type ListenerConfig struct {
	Host string `json:"host,omitempty"`
	Port string `json:"port,omitempty"`
	WS   bool   `json:"enable_ws,omitempty"`
}

//Current configuration struct, maxQueueDepth sets the maximum number of unacknowledged mesages
//for a client. Listeners is a slice of ListenerConfigs
type ConfigObject struct {
	MaxQueueDepth int               `json:"maxQueueDepth"`
	Listeners     []*ListenerConfig `json:"listeners"`
	Logging       struct {
		Info     string `json:"info"`
		Protocol string `json:"protocol"`
		Errlog   string `json:"error"`
		Debug    string `json:"debug"`
	}
}

func (c *ConfigObject) GetLogTarget(log string) io.Writer {
	switch log {
	case "info":
		if target, ok := logTargets[c.Logging.Info]; ok {
			return target
		} else {
			return os.Stdout
		}
	case "protocol":
		if target, ok := logTargets[c.Logging.Protocol]; ok {
			return target
		} else {
			return ioutil.Discard
		}
	case "error":
		if target, ok := logTargets[c.Logging.Errlog]; ok {
			return target
		} else {
			return os.Stderr
		}
	case "debug":
		if target, ok := logTargets[c.Logging.Debug]; ok {
			return target
		} else {
			return ioutil.Discard
		}
	}
	return nil
}

func ParseConfig(confFile string, confVar *ConfigObject) error {
	file, err := os.Open(confFile)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)

	err = decoder.Decode(confVar)
	if err != nil {
		return err
	}
	return nil
}

//set up the endpoints for the various loggers, needs to be made configurable.
func ConfigureLogger(infoHandle io.Writer, protocolHandle io.Writer, errorHandle io.Writer, debugHandle io.Writer) {
	INFO = log.New(infoHandle,
		"INFO: ",
		log.Ldate|log.Ltime)

	PROTOCOL = log.New(protocolHandle,
		"PROTOCOL: ",
		log.Ldate|log.Ltime)

	ERROR = log.New(errorHandle,
		"ERROR: ",
		log.Ldate|log.Ltime|log.Lshortfile)

	DEBUG = log.New(debugHandle,
		"DEBUG: ",
		log.Ldate|log.Ltime|log.Lshortfile)
}
