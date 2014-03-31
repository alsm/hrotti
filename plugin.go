package main

type Plugin interface {
	Init() error
	AddSub(*Client, []string, byte, chan bool)
	DeleteSub(*Client, chan bool)
}
