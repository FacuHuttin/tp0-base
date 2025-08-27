package common

import (
	"bufio"
	"fmt"
	"net"
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
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) *Client {
	client := &Client{
		config: config,
	}
	return client
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}
	c.conn = conn
	return nil
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop(signalChan <-chan os.Signal) {
	// There is an autoincremental msgID to identify every message sent
	// Messages if the message amount threshold has not been surpassed
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		// Check if signal received (graceful shutdown requested)
		select {
		case sig := <-signalChan:
			log.Infof("action: signal_received | signal: %v | result: stopping_loop | client_id: %v | last_msg_id: %v",
				sig, c.config.ID, msgID-1)
			return
		default:
			// Continue with normal operation
		}

		// Create the connection the server in every loop iteration. Send an
		c.createClientSocket()

		// TODO: Modify the send to avoid short-write
		fmt.Fprintf(
			c.conn,
			"[CLIENT %v] Message N°%v\n",
			c.config.ID,
			msgID,
		)
		msg, err := bufio.NewReader(c.conn).ReadString('\n')
		c.conn.Close()

		if err != nil {
			log.Errorf("action: receive_message | result: fail | client_id: %v | error: %v",
				c.config.ID,
				err,
			)
			return
		}

		log.Infof("action: receive_message | result: success | client_id: %v | msg: %v",
			c.config.ID,
			msg,
		)

		// Wait a time between sending one message and the next one
		// but also check for signals
		sleepInterval := 100 * time.Millisecond
		totalSleep := time.Duration(0)
		for totalSleep < c.config.LoopPeriod {
			select {
			case sig := <-signalChan:
				log.Infof("action: signal_received | signal: %v | result: stopping_loop | client_id: %v | completed_msg_id: %v",
					sig, c.config.ID, msgID)
				return
			case <-time.After(sleepInterval):
				totalSleep += sleepInterval
			}
		}
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}
