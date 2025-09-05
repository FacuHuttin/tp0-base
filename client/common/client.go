package common

import (
	"fmt"
	"os"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

type ClientConfig struct {
	ID             string
	ServerAddress  string
	LoopAmount     int
	LoopPeriod     time.Duration
	BatchMaxAmount int
	BetsFile       string
	MaxRetries     int
	BaseSleep      time.Duration
	SleepInterval  time.Duration
	Timeout        time.Duration
}

type Client struct {
	config  ClientConfig
	service *AgencyService
}

func NewClient(config ClientConfig) (*Client, error) {

	agencyInfo, err := NewAgencyInfo(config.ID, config.BatchMaxAmount, config.BetsFile)
	if err != nil {
		return nil, fmt.Errorf("error creating agency info: %v", err)
	}

	conn := NewTCPConnection(config.ServerAddress)
	service := NewAgencyService(agencyInfo, conn, config.LoopAmount, config.LoopPeriod)

	client := &Client{
		service: service,
		config:  config,
	}

	return client, nil
}

func (c *Client) StartClientLoop(signalChan <-chan os.Signal) int {
	// Check for signal at the beginning of each iteration
	select {
	case sig := <-signalChan:
		log.Infof("action: signal_received | result: success | client_id: %v | signal: %v",
			c.config.ID, sig)
		return 0 // Graceful shutdown
	default:
		// Continue with normal operation
	}

	// Step 1: Connect and send all bets
	if err := c.connectWithSleep(signalChan); err != nil {
		if err.Error() == "client interrupted during sleep" {
			return 0 // Graceful shutdown during sleep
		}
		log.Criticalf("action: connect_with_sleep | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return 2 // Not able to connect after retries
	}

	if err := c.service.ProcessCommunication(signalChan); err != nil {
		if err.Error() == "client interrupted" {
			return 0 // Graceful shutdown during communication
		}
		log.Errorf("action: process_communication | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		c.service.connection.Close() // Close connection on error
		return 3                     // Error during communication
	}

	log.Infof("action: step1_completed | result: success | client_id: %v", c.config.ID)

	// Step 2: Query for winners using the same connection
	if err := c.queryWinnersWithPersistentConnection(signalChan); err != nil {
		if err.Error() == "client interrupted during winners query" || err.Error() == "client interrupted during backoff" {
			c.service.connection.Close() // Close on error or interruption
			return 0                     // Graceful shutdown
		}
		log.Errorf("action: query_winners | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		c.service.connection.Close() // Close on error
		return 4                     // Error during winners query
	}

	// Close connection after completing all steps
	c.service.connection.Close()

	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
	return 0 // Normal exit
}

func (c *Client) queryWinnersWithPersistentConnection(signalChan <-chan os.Signal) error {
	sleepDuration := 0 * time.Second
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		select {
		case <-signalChan:
			log.Infof("action: signal_received | result: success | client_id: %v",
				c.config.ID)
			return fmt.Errorf("client interrupted during winners query")
		default:
		}

		log.Infof("action: winners_query_attempt | result: in_progress | client_id: %v | attempt: %d",
			c.config.ID, attempt+1)

		// Use existing connection (no reconnection needed)
		err := c.service.ProcessWinnersQuery(signalChan, attempt, sleepDuration)

		if err == nil {
			// Success!
			return nil
		}

		if err.Error() == "client interrupted during winners query" || err.Error() == "client interrupted during backoff" {
			return err
		}

		if attempt == c.config.MaxRetries {
			return fmt.Errorf("winners query failed after %d attempts", c.config.MaxRetries+1)
		}

		sleepDuration = c.config.BaseSleep * time.Duration(1<<attempt)

		log.Debugf("action: winners_query_retry | result: in_progress | client_id: %v | sleep: %v",
			c.config.ID, sleepDuration)

		if c.sleepWithSignalCheck(signalChan, sleepDuration) {
			return fmt.Errorf("client interrupted during backoff")
		}
	}

	return fmt.Errorf("winners query failed after %d attempts", c.config.MaxRetries+1)
}

func (c *Client) sleepWithSignalCheck(signalChan <-chan os.Signal, sleepPeriod time.Duration) bool {
	totalSleep := time.Duration(0)
	for totalSleep < sleepPeriod {
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received_during_sleep | result: success | client_id: %v | signal: %v",
				c.config.ID, sig)
			return true
		case <-time.After(c.config.SleepInterval):
			totalSleep += c.config.SleepInterval
		}
	}
	return false
}

func (c *Client) connectWithSleep(signalChan <-chan os.Signal) error {
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		log.Infof("action: connect_attempt | result: in_progress | client_id: %v | attempt: %v | timeout: %v",
			c.config.ID, attempt+1, c.config.Timeout)

		err := c.service.connection.ConnectWithTimeout(c.config.Timeout)
		if err == nil {

			select {
			case sig := <-signalChan:
				log.Infof("action: signal_received | result: success | client_id: %v | signal: %v",
					c.config.ID, sig)
				return fmt.Errorf("client interrupted during sleep")
			default:
				// Continue with normal operation
			}

			log.Infof("action: connect | result: success | client_id: %v | attempt: %v",
				c.config.ID, attempt+1)
			return nil // Success
		}

		log.Infof("action: connect_attempt | result: fail | client_id: %v | attempt: %v | error: %v",
			c.config.ID, attempt+1, err)

		if attempt == c.config.MaxRetries {
			return err
		}

		sleepDuration := c.config.BaseSleep * time.Duration(attempt+1)

		log.Debugf("action: connect_retry | result: in_progress | client_id: %v | attempt: %v | sleep: %v",
			c.config.ID, attempt+1, sleepDuration)

		if c.sleepWithSignalCheck(signalChan, sleepDuration) {
			return fmt.Errorf("client interrupted during sleep")
		}
	}

	return nil
}
