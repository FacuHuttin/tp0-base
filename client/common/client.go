package common

import (
	"os"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
	Name          string
	Surname       string
	DNI           string
	Birthday      string
	BetNumber     string
}

// Client Entity that encapsulates how
type Client struct {
	config  ClientConfig
	service *AgencyService
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) *Client {
	agencyInfo := &AgencyInfo{
		ID:        config.ID,
		Name:      config.Name,
		Surname:   config.Surname,
		DNI:       config.DNI,
		Birthday:  config.Birthday,
		BetNumber: config.BetNumber,
	}

	conn := NewTCPConnection(config.ServerAddress)
	service := NewAgencyService(agencyInfo, conn, config.LoopAmount, config.LoopPeriod)

	client := &Client{
		service: service,
		config:  config,
	}

	return client
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop(signalChan <-chan os.Signal) int {
	// There is an autoincremental msgID to identify every message sent
	// Messages if the message amount threshold has not been surpassed
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		// Check for signal at the beginning of each iteration
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received | result: success | client_id: %v | last_msg_id: %v | signal: %v",
				c.config.ID, msgID-1, sig)
			return 0 // Graceful shutdown
		default:
			// Continue with normal operation
		}

		// Connect for message
		if err := c.service.connection.Connect(); err != nil {
			log.Criticalf("action: connect | result: fail | client_id: %v | error: %v",
				c.config.ID, err)
			return 1 // Not able to connect
		}

		// Check for signal before starting communication
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received | result: success | client_id: %v | msg_id: %v | signal: %v",
				c.config.ID, msgID, sig)
			c.service.connection.Close() // Clean up connection
			return 0 // Graceful shutdown
		default:
			// Continue with communication
		}

		if err := c.service.ProcessCommunication(msgID); err != nil {
			log.Errorf("action: process_communication | result: fail | client_id: %v | error: %v",
				c.config.ID, err)
			c.service.connection.Close() // Close connection on error
			return 2 // Error during communication
		}

		// Close connection after successful communication
		c.service.connection.Close()

		// Check for signal after communication but before sleep
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received | result: success | client_id: %v | completed_msg_id: %v | signal: %v",
				c.config.ID, msgID, sig)
			return 0 // Graceful shutdown
		default:
			// Continue to sleep
		}

		// Sleep with signal checking
		if c.sleepWithSignalCheck(signalChan, msgID) {
			// Signal received during sleep, exit gracefully
			return 0
		}
	}

	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
	return 0 // Normal exit
}

func (c *Client) sleepWithSignalCheck(signalChan <-chan os.Signal, msgID int) bool {
	sleepInterval := 100 * time.Millisecond
	totalSleep := time.Duration(0)
	for totalSleep < c.config.LoopPeriod {
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received_during_sleep | result: success | client_id: %v | completed_msg_id: %v | signal: %v",
				c.config.ID, msgID, sig)
			return true // Signal received during sleep
		case <-time.After(sleepInterval):
			totalSleep += sleepInterval
		}
	}
	return false // No signal received
}
