package common

import (
	"fmt"
	"math"
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

	// Connect for message with exponential backoff
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

	// Close connection after successful communication
	c.service.connection.Close()

	// Check for signal after communication but before sleep
	select {
	case sig := <-signalChan:
		log.Infof("action: signal_received | result: success | client_id: %v | signal: %v",
			c.config.ID, sig)
		return 0 // Graceful shutdown
	default:
		// Continue to sleep
	}

	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
	return 0 // Normal exit
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

// connectWithBackoff attempts to connect with exponential backoff
func (c *Client) connectWithBackoff(signalChan <-chan os.Signal) error {
	baseBackoff := c.config.BaseBackoff
	if baseBackoff == 0 {
		baseBackoff = 100 * time.Millisecond // Default base backoff
	}

	maxRetries := c.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 5 // Default max retries
	}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Try to connect
		err := c.service.connection.Connect()
		if err == nil {
			return nil // Success
		}

		// If this was the last attempt, return the error
		if attempt == maxRetries {
			return err
		}

		// Calculate backoff duration: base * 2^attempt
		backoffDuration := time.Duration(float64(baseBackoff) * math.Pow(2, float64(attempt)))

		log.Infof("action: connect_retry | result: scheduled | client_id: %v | attempt: %v | backoff: %v",
			c.config.ID, attempt+1, backoffDuration)

		// Wait with exponential backoff
		if c.sleepWithSignalCheck(signalChan, backoffDuration) {
			return fmt.Errorf("client interrupted during backoff")
		}
	}

	return nil
}
