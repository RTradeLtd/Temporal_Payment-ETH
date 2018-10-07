package main

import (
	"errors"
	"log"
	"os"

	request "github.com/RTradeLtd/Temporal_Payment-ETH/gapi/request"
	"github.com/RTradeLtd/config"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	if len(os.Args) > 2 || len(os.Args) < 2 {
		err := errors.New("invalid invocation, ./gapi <server>")
		log.Fatal(err)
	}
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		err := errors.New("CONFIG_PATH env var is empty")
		log.Fatal(err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	switch os.Args[1] {
	case "server":
		generateServerAndList("127.0.0.1:9090", "tcp", cfg)
	case "client":
		req := &request.SignRequest{
			Address:      common.HexToAddress("0").String(),
			Method:       "0",
			Number:       "0",
			ChargeAmount: "1",
		}
		generateClient("127.0.0.1:9090", false, req)
	default:
		err := errors.New("argument nto supported")
		log.Fatal(err)
	}

}
