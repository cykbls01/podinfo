package main

import (
	"github.com/stefanprodan/podinfo/test"
	"log"
)

func main() {
	if err := test.Run(); err != nil {
		log.Fatalln(err)
	}
}
