package common

import (
	"os"
	"os/signal"
	"syscall"
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
	BatchMaxAmount int
}

// Client used by the LotteryAgency to send bets to the NationalLotteryCenter (server)
type Client struct {
	config   ClientConfig
	socket   *TCPConn
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	socket, err := NewTCPConn(c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
	}
	c.socket = socket
	return nil
}

func (c *Client) terminate_loop() {
	if c.socket != nil {
		c.socket.Close()
	}
	log.Infof("action: close_connection_with_server | result: success | client_id: %v",
		c.config.ID,
	)
}

// Custom min function
func min(a, b int) int {
	if a < b {
			return a
	}
	return b
}

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop(bets []Bet) {

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGTERM)
	
	for i := 0; i < len(bets); {
		err := c.createClientSocket()
		if err != nil {
			return
		}
		select {
		case <-sigChannel:
			c.terminate_loop()
			return
		default:
			// Create a batch of bets
			batchSize := min(c.config.BatchMaxAmount, len(bets)-i)
			batch := bets[i : i+batchSize]
			i += batchSize

			// Encode the batch of bets
			encodedBatch, err := encode_batch_message(batch, c.config.ID)
			if err != nil {
				log.Errorf("action: encode_batch_message | result: fail | client_id: %v | error: %v",
					c.config.ID,
					err,
				)
				continue
			}

			_, err = c.socket.WriteFull(encodedBatch)
			if err != nil {
				log.Errorf("action: write_batch_message_to_server | result: fail | client_id: %v | error: %v",
					c.config.ID,
					err,
				)
				c.terminate_loop()
				return
			}
			select {
			case <-sigChannel:
				c.terminate_loop()
				return
			default:
				buffer := make([]byte, 1)
				_, err := c.socket.ReadFull(buffer)
		
				if err != nil {
					log.Errorf("action: receive_confirmation_message | result: fail | client_id: %v | error: %v",
						c.config.ID,
						err,
					)
					return
				}

				err = decode_confirmation_message(buffer)

				if err != nil {
					log.Errorf("action: decode_confirmation_message | result: fail | client_id: %v | error: %v",
						c.config.ID,
						err,
					)
					continue
				}

				log.Infof("action: batch_sent | result: success | client_id: %v | batch_size: %d",
					c.config.ID,
					batchSize,
				)
				select {
				case <-sigChannel:
					return
				case <-time.After(c.config.LoopPeriod):
					c.socket.Close()
				}
			}
		}
	}
	log.Infof("action: send_bets | result: success | client_id: %v", c.config.ID)
}
