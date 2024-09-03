package common

import (
	"bufio"
	"fmt"
	"net"
	"time"
	"os"
	"os/signal"
	"syscall"
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

func (c *Client) terminate_loop() {
	if c.conn != nil {
			c.conn.Close()
	}
	log.Infof("action: close_connection_with_server | result: success | client_id: %v",
			c.config.ID,
	)
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop() {

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGTERM)

	// There is an autoincremental msgID to identify every message sent
	// Messages if the message amount threshold has not been surpassed
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		select {
		case <-sigChannel:
			c.terminate_loop()
			return
		default:
			// Create the connection the server in every loop iteration. Send an
			c.createClientSocket()
	
			// TODO: Modify the send to avoid short-write
			fmt.Fprintf(
				c.conn,
				"[CLIENT %v] Message N°%v\n",
				c.config.ID,
				msgID,
			)
			select {
			case <-sigChannel:
				c.terminate_loop()
				return
			default:
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
				select {
				case <-sigChannel:
					return
				case <-time.After(c.config.LoopPeriod):
					// Wait a time between sending one message and the next one
				}
			}
		}
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}
