package main

import (
	"encoding/json"
	"os"
)

type ListenerConfig struct {
	Host string `json:"host,omitempty"`
	Port string `json:"port,omitempty"`
	WS   bool   `json:"enable_ws,omitempty"`
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
