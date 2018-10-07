package main

import (
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"strconv"

	request "github.com/RTradeLtd/Temporal_Payment-ETH/gapi/request"
	response "github.com/RTradeLtd/Temporal_Payment-ETH/gapi/response"
	pb "github.com/RTradeLtd/Temporal_Payment-ETH/gapi/service"
	"github.com/RTradeLtd/Temporal_Payment-ETH/signer"
	"github.com/RTradeLtd/config"
	"github.com/ethereum/go-ethereum/common"
	context "golang.org/x/net/context"
	"google.golang.org/grpc"
)

func generateServerAndList(listenAddr, protocol string, cfg *config.TemporalConfig) {
	lis, err := net.Listen(protocol, listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer lis.Close()
	gServer := grpc.NewServer()
	ps, err := signer.GeneratePaymentSigner(cfg)
	server := &Server{
		PS: ps,
	}
	pb.RegisterSignerServer(gServer, server)
	gServer.Serve(lis)
}

type Server struct {
	PS *signer.PaymentSigner
}

func (s *Server) GetSignedMessage(ctx context.Context, req *request.SignRequest) (*response.SignResponse, error) {
	fmt.Println("message received, processing...")
	addr := req.Address
	method := req.Method
	number := req.Number
	chargeAmount := req.ChargeAmount
	addrTyped := common.HexToAddress(addr)
	methodUint64, err := strconv.ParseUint(method, 10, 64)
	if err != nil {
		return nil, err
	}
	methodUint8 := uint8(methodUint64)
	numberBig, valid := new(big.Int).SetString(number, 10)
	if !valid {
		return nil, errors.New("failed to convert payment number to big int")
	}
	chargeAmountBig, valid := new(big.Int).SetString(chargeAmount, 10)
	if !valid {
		return nil, errors.New("failed to convert charge amount to big")
	}
	msg, err := s.PS.GenerateSignedPaymentMessagePrefixed(
		addrTyped, methodUint8, numberBig, chargeAmountBig,
	)
	if err != nil {
		return nil, err
	}
	fmt.Printf("%+v\n", msg)
	res := &response.SignResponse{}
	fmt.Println("processing finished")
	return res, nil
}
