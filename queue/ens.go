package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/RTradeLtd/Pay/ethereum"
	"github.com/RTradeLtd/Pay/log"
	"github.com/RTradeLtd/database/v2/models"
	"github.com/streadway/amqp"
)

// ProcessENSRequest is used to process ens requests
func (qm *Manager) ProcessENSRequest(
	ctx context.Context,
	wg *sync.WaitGroup,
	msgs <-chan amqp.Delivery,
) error {
	var connectionType string
	if qm.cfg.Ethereum.Connection.INFURA.URL != "" {
		connectionType = "infura"
	}
	if qm.cfg.Ethereum.Connection.RPC.IP != "" &&
		qm.cfg.Ethereum.Connection.RPC.Port != "" {
		connectionType = "rpc"
	}
	ethclient, err := ethereum.NewClient(qm.cfg, connectionType)
	if err != nil {
		return err
	}
	if err := ethclient.UnlockAccountFromConfig(qm.cfg); err != nil {
		return err
	}
	if err := ethclient.SetResolver(ethereum.TemporalENSName); err != nil {
		return err
	}
	logger, err := log.NewLogger(qm.cfg.LogDir+"pay_ens_email_publisher.log", false)
	if err != nil {
		return err
	}
	qmEmail, err := New(EmailSendQueue, qm.cfg, logger, true)
	if err != nil {
		return err
	}
	usg := models.NewUsageManager(qm.db)
	userm := models.NewUserManager(qm.db)
	qm.l.Info("processing ens requests")
	for {
		select {
		case d := <-msgs:
			wg.Add(1)
			go qm.processENSRequest(d, wg, usg, userm, qmEmail, ethclient)
		case <-ctx.Done():
			qm.Close()
			wg.Done()
			return nil
		case msg := <-qm.ErrCh:
			qm.Close()
			wg.Done()
			qm.l.Errorw(
				"a protocol connection error stopping rabbitmq was received",
				"error", msg.Error())
			return errors.New(ErrReconnect)
		}
	}
}

func (qm *Manager) processENSRequest(
	d amqp.Delivery,
	wg *sync.WaitGroup,
	usage *models.UsageManager,
	userm *models.UserManager,
	qmEmail *Manager,
	ec *ethereum.Client,
) {
	defer wg.Done()
	qm.l.Info("new ens request message received")
	req := ENSRequest{}
	if err := json.Unmarshal(d.Body, &req); err != nil {
		qm.l.Error("failed to unmarshal message")
		d.Ack(false)
		return
	}
	var err error
	switch req.Type {
	case ENSRegisterSubName:
		qm.l.Info("registering sub domain")
		err = ec.RegisterSubDomain(req.UserName, ethereum.TemporalENSName)
	case ENSUpdateContentHash:
		qm.l.Info("updating content hash")
		err = ec.UpdateContentHash(
			req.UserName,
			ethereum.TemporalENSName,
			req.ContentHash,
		)
	case ENSRegisterName:
		qm.l.Info("registering name")
		// TODO(bonedaddy): re-enable
		// err = ec.RegisterName(req.UserName + ".eth")
		fallthrough
	default:
		qm.l.Errorw("unsupported request type", "user", req.UserName, "type", req.Type)
		d.Ack(false)
		return
	}
	qm.l.Info("searching for user")
	user, usrErr := userm.FindByUserName(req.UserName)
	if usrErr != nil {
		// if we cant find the user, email admin for help
		qm.l.Errorw("failed to search for user", "user", req.UserName, "type", req.Type)
		qm.l.Warn("sending email to admin instead")
		user = &models.User{
			UserName:     "admin",
			EmailAddress: "admin@rtradetechnologies.com",
			EmailEnabled: true,
		}
	}
	if !user.EmailEnabled {
		qm.l.Info("successfully processed ens request")
		d.Ack(false)
		return
	}
	qm.l.Info("sending ens request confirmation email")
	var es EmailSend
	if err != nil {
		es = EmailSend{
			Subject:     "ENS Request Processing Failure",
			Content:     fmt.Sprint("Your ens request failed due to the following error: ", err),
			ContentType: "text/html",
			UserNames:   []string{req.UserName},
			Emails:      []string{user.EmailAddress},
		}
		qm.l.Errorw(
			"failed to process ens request",
			"user", req.UserName,
			"type", req.Type,
			"error", err,
		)
	} else {
		es = EmailSend{
			Subject:     "ENS Request Processed Successfully",
			Content:     fmt.Sprintf("your ens request was successfully processed"),
			ContentType: "text/html",
			UserNames:   []string{req.UserName},
			Emails:      []string{user.EmailAddress},
		}
	}
	if err := qmEmail.PublishMessage(es); err != nil {
		qm.l.Errorw("failed to send ens request confirmation email", "error", err)
	}
	qm.l.Info("successfully processed ens request")
	d.Ack(false)
	return
}
