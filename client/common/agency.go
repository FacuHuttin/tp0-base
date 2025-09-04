package common

import (
	"fmt"
)

type AgencyInfo struct {
	ID        string
	Name      string
	Surname   string
	DNI       string
	Birthday  string
	BetNumber string
}

type Message struct {
	ID       int
	Content  []byte
	ClientID string
}

func (c *AgencyInfo) CreateBetMessage(messageID int) (*Message, error) {
	bet, err := SerializeBetMessage(c)
	if err != nil {
		return nil, fmt.Errorf("error serializing agency info: %v", err)
	}

	return &Message{
		ID:       messageID,
		Content:  bet,
		ClientID: c.ID,
	}, nil
}

// AgencyService handles the communication logic for the agency
type AgencyService struct {
	agencyInfo *AgencyInfo
	connection Connection
}

func NewAgencyService(agencyInfo *AgencyInfo, conn Connection) *AgencyService {
	return &AgencyService{
		agencyInfo: agencyInfo,
		connection: conn,
	}
}

func (cs *AgencyService) SendMessage(message *Message) error {
	return SendBetMessage(cs.connection, message)
}

func (cs *AgencyService) ProcessCommunication(msgID int) error {
	message, err := cs.agencyInfo.CreateBetMessage(msgID)
	if err != nil {
		return err
	}

	if err := cs.SendMessage(message); err != nil {
		return err
	}

	ackMessage, err := cs.ReceiveAckMessage()
	if err != nil {
		return err
	}

	log.Infof("action: apuesta_enviada | result: success | dni: %s | numero: %s",
		ackMessage.DNI, ackMessage.BetNumber)

	if err := cs.SendCloseMessage(); err != nil {
		return err
	}

	return nil
}

func (cs *AgencyService) ReceiveAckMessage() (*AckBetMessage, error) {
	return ReceiveAckMessage(cs.connection)
}

func (cs *AgencyService) SendCloseMessage() error {
	return SendCloseMessage(cs.connection)
}
