package common

import (
	"fmt"
	"os"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID             string
	ServerAddress  string
	LoopAmount     int
	LoopPeriod     time.Duration
	BatchMaxAmount int
	BetsFile       string
	MaxRetries     int           // New: Maximum retry attempts
	BaseBackoff    time.Duration // New: Base backoff duration
}

// Client Entity that encapsulates how
type Client struct {
	config  ClientConfig
	service *AgencyService
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
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

// StartClientLoop Send messages to the client until some time threshold is met
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

	// Step 1: Connect and send all bets with exponential backoff
	if err := c.connectWithBackoff(signalChan); err != nil {
		if err.Error() == "client interrupted during backoff" {
			return 0 // Graceful shutdown during backoff
		}
		log.Criticalf("action: connect_with_backoff | result: fail | client_id: %v | error: %v",
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

	// Close connection after step 1
	c.service.connection.Close()

	log.Infof("action: step1_completed | result: success | client_id: %v", c.config.ID)

	// Step 2: Query for winners with exponential backoff
	if err := c.queryWinnersWithBackoff(signalChan); err != nil {
		if err.Error() == "client interrupted during winners query" || err.Error() == "client interrupted during backoff" {
			return 0 // Graceful shutdown
		}
		log.Errorf("action: query_winners | result: fail | client_id: %v | error: %v",
			c.config.ID, err)
		return 4 // Error during winners query
	}

	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
	return 0 // Normal exit
}

// queryWinnersWithBackoff handles step 2: querying for winners with reconnection logic
func (c *Client) queryWinnersWithBackoff(signalChan <-chan os.Signal) error {
	maxRetries := c.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 10 // Default max retries for winners query
	}

	baseBackoff := c.config.BaseBackoff
	if baseBackoff == 0 {
		baseBackoff = 1 * time.Second // Default base backoff for winners query
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-signalChan:
			log.Infof("action: signal_received | result: success | client_id: %v",
				c.config.ID)
			return fmt.Errorf("client interrupted during winners query")
		default:
		}

		log.Infof("action: winners_query_connection_attempt | result: in_progress | client_id: %v | attempt: %d",
			c.config.ID, attempt+1)

		// Create new connection for step 2
		conn := NewTCPConnection(c.config.ServerAddress)
		c.service.connection = conn

		// Try to connect for winners query
		connectionTimeout := baseBackoff * 5
		if err := conn.ConnectWithTimeout(connectionTimeout); err != nil {

			if attempt == maxRetries {
				return fmt.Errorf("failed to connect for winners query after %d attempts", maxRetries+1)
			}

			// Sleep before retrying connection
			sleepDuration := baseBackoff * time.Duration(attempt+1)
			log.Infof("action: winners_query_connection_retry | result: scheduled | client_id: %v | sleep: %v",
				c.config.ID, sleepDuration)

			if c.sleepWithSignalCheck(signalChan, sleepDuration) {
				return fmt.Errorf("client interrupted during backoff")
			}
			continue
		}

		log.Infof("action: winners_query_connect | result: success | client_id: %v | attempt: %d",
			c.config.ID, attempt+1)

		// Try to query winners
		err := c.service.ProcessWinnersQuery(signalChan, 0, baseBackoff) // 0 retries as we handle retries here
		conn.Close()

		if err == nil {
			// Success!
			return nil
		}

		if err.Error() == "client interrupted during winners query" || err.Error() == "client interrupted during backoff" {
			return err
		}

		// If this was the last attempt, return the error
		if attempt == maxRetries {
			return fmt.Errorf("winners query failed after %d attempts", maxRetries+1)
		}

		// Sleep with exponential backoff before next attempt
		sleepDuration := baseBackoff * time.Duration(attempt+1)
		log.Infof("action: winners_query_retry | result: scheduled | client_id: %v | sleep: %v",
			c.config.ID, sleepDuration)

		if c.sleepWithSignalCheck(signalChan, sleepDuration) {
			return fmt.Errorf("client interrupted during backoff")
		}
	}

	return fmt.Errorf("winners query failed after %d attempts", maxRetries+1)
}

func (c *Client) sleepWithSignalCheck(signalChan <-chan os.Signal, sleepPeriod time.Duration) bool {
	sleepInterval := 100 * time.Millisecond
	totalSleep := time.Duration(0)
	for totalSleep < sleepPeriod {
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received_during_sleep | result: success | client_id: %v | signal: %v",
				c.config.ID, sig)
			return true // Signal received during sleep
		case <-time.After(sleepInterval):
			totalSleep += sleepInterval
		}
	}
	return false // No signal received
}

// connectWithBackoff attempts to connect with fixed timeout and incremental backoff sleep
func (c *Client) connectWithBackoff(signalChan <-chan os.Signal) error {
	baseBackoff := c.config.BaseBackoff
	if baseBackoff == 0 {
		baseBackoff = 100 * time.Millisecond // Default base backoff
	}

	maxRetries := c.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 5 // Default max retries
	}

	// Fixed timeout for connection attempts
	connectionTimeout := baseBackoff * 5 // Use 5x baseBackoff as fixed timeout

	for attempt := 0; attempt <= maxRetries; attempt++ {
		log.Infof("action: connect_attempt | result: in_progress | client_id: %v | attempt: %v | timeout: %v",
			c.config.ID, attempt+1, connectionTimeout)

		// Try to connect with fixed timeout
		err := c.service.connection.ConnectWithTimeout(connectionTimeout)
		if err == nil {
			select {
			case sig := <-signalChan:
				log.Infof("action: signal_received | result: success | client_id: %v | signal: %v",
					c.config.ID, sig)
				return fmt.Errorf("client interrupted during backoff")
			default:
				// Continue with normal operation
			}
			log.Infof("action: connect | result: success | client_id: %v | attempt: %v",
				c.config.ID, attempt+1)
			return nil // Success
		}

		log.Infof("action: connect_attempt | result: fail | client_id: %v | attempt: %v | error: %v",
			c.config.ID, attempt+1, err)

		// If this was the last attempt, return the error
		if attempt == maxRetries {
			return err
		}

		// Calculate incremental sleep duration: baseBackoff * (attempt + 1)
		sleepDuration := baseBackoff * time.Duration(attempt+1)

		log.Infof("action: connect_retry | result: scheduled | client_id: %v | attempt: %v | sleep: %v",
			c.config.ID, attempt+1, sleepDuration)

		// Wait with incremental sleep
		if c.sleepWithSignalCheck(signalChan, sleepDuration) {
			return fmt.Errorf("client interrupted during backoff")
		}
	}

	return nil
}
