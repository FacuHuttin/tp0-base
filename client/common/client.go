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

// StartClientLoop Send messages to the client until some time threshold is met
func (c *Client) StartClientLoop(bets []Bet) {

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGTERM)

	err := c.createClientSocket()
	if err != nil {
		return
	}

	for i := 0; i < len(bets); i++ {
		select {
		case <-sigChannel:
			c.terminate_loop()
			return
		default:
			bet := bets[i]
			encodedBet, err := encode_bet_message(&bet, c.config.ID)
			if err != nil {
				log.Errorf("action: encode_bet_message | result: fail | client_id: %v | error: %v",
					c.config.ID,
					err,
				)
				continue
			}
			_, err = c.socket.WriteFull(encodedBet)
			if err != nil {
				log.Errorf("action: write_bet_message_to_server | result: fail | client_id: %v | error: %v",
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

				log.Infof("action: apuesta_enviada | result: success | dni: %d | numero: %d",
					bet.DNI,
					bet.LotteryNum,
				)
				select {
				case <-sigChannel:
					return
				case <-time.After(c.config.LoopPeriod):
				}
			}
		}
	}
	log.Infof("action: send_bets | result: success | client_id: %v", c.config.ID)
}
