package main

import (
	"time"
	"tracing/client"
	"tracing/server"
)

func main() {

	go server.Run()

	time.Sleep(time.Second * 1)

	client.Client()

	select {}
}
