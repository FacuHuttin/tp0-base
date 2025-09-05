package common

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"time"
)

type Bet struct {
	Name      string
	Surname   string
	DNI       string
	Birthday  string
	BetNumber string
}

type AgencyInfo struct {
	ID             string
	BatchMaxAmount int
	BetsFile       string
	totalBets      int
	batchCount     int
	currentOffset  int
}

type Message struct {
	ID       int
	Content  []byte
	ClientID string
}

func NewAgencyInfo(id string, batchMaxAmount int, betsFile string) (*AgencyInfo, error) {
	agency := &AgencyInfo{
		ID:             id,
		BatchMaxAmount: batchMaxAmount,
		BetsFile:       betsFile,
		currentOffset:  0,
	}

	if err := agency.countTotalBets(); err != nil {
		return nil, fmt.Errorf("error counting bets from file: %v", err)
	}

	return agency, nil
}

func (c *AgencyInfo) countTotalBets() error {
	file, err := os.Open(c.BetsFile)
	if err != nil {
		return fmt.Errorf("error opening bets file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading CSV file: %v", err)
	}

	if lineCount == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	c.totalBets = lineCount
	c.batchCount = (lineCount + c.BatchMaxAmount - 1) / c.BatchMaxAmount

	return nil
}

func (c *AgencyInfo) readNextBatch() ([]Bet, error) {
	file, err := os.Open(c.BetsFile)
	if err != nil {
		return nil, fmt.Errorf("error opening bets file: %v", err)
	}
	defer file.Close()

	if c.currentOffset >= c.totalBets {
		return nil, fmt.Errorf("no more bets to read")
	}

	reader := csv.NewReader(file)
	var batch []Bet

	for i := 0; i < c.currentOffset; i++ {
		_, err := reader.Read()
		if err != nil {
			return nil, fmt.Errorf("error skipping to offset %d: %v", c.currentOffset, err)
		}
	}

	end := c.currentOffset + c.BatchMaxAmount
	if end > c.totalBets {
		end = c.totalBets
	}

	for i := c.currentOffset; i < end; i++ {
		record, err := reader.Read()
		if err != nil {
			return nil, fmt.Errorf("error reading CSV record at line %d: %v", i+1, err)
		}

		if len(record) != 5 {
			return nil, fmt.Errorf("invalid CSV format at row %d: expected 5 columns (Name,Surname,DNI,Birthday,BetNumber), got %d", i+1, len(record))
		}

		for j, field := range record {
			if field == "" {
				fieldNames := []string{"Name", "Surname", "DNI", "Birthday", "BetNumber"}
				return nil, fmt.Errorf("invalid CSV format at row %d: %s cannot be empty", i+1, fieldNames[j])
			}
		}

		bet := Bet{
			Name:      record[0],
			Surname:   record[1],
			DNI:       record[2],
			Birthday:  record[3],
			BetNumber: record[4],
		}
		batch = append(batch, bet)
	}

	c.currentOffset = end
	return batch, nil
}

type AgencyService struct {
	agencyInfo *AgencyInfo
	connection Connection
	loopAmount int
	loopPeriod time.Duration
}

func NewAgencyService(agencyInfo *AgencyInfo, conn Connection, loopAmount int, loopPeriod time.Duration) *AgencyService {
	return &AgencyService{
		agencyInfo: agencyInfo,
		connection: conn,
		loopAmount: loopAmount,
		loopPeriod: loopPeriod,
	}
}

func (cs *AgencyService) ProcessCommunication(signalChan <-chan os.Signal) error {
	for batchIndex := 0; batchIndex < cs.agencyInfo.batchCount; batchIndex++ {
		// Check for signal before each batch
		select {
		case <-signalChan:
			log.Debugf("action: shutdown_signal_received | result: in_progress | client_id: %v | batch: %d",
				cs.agencyInfo.ID, batchIndex)
			if err := cs.SendCloseMessage(); err != nil {
				return fmt.Errorf("error sending close message: %v", err)
			}
			return fmt.Errorf("client interrupted")
		default:
		}

		// Get the batch to send
		batch, err := cs.agencyInfo.GetBatch(batchIndex)
		if err != nil {
			return fmt.Errorf("error getting batch %d: %v", batchIndex, err)
		}

		// Send batch bet message
		batchNumber := batchIndex + 1

		if err := SendBatchBetMessage(cs.connection, cs.agencyInfo.ID, batchNumber, batch); err != nil {
			return fmt.Errorf("error sending batch %d: %v", batchNumber, err)
		}

		// Receive ACK for this batch
		ackMessage, err := cs.ReceiveAckMessage()
		if err != nil {
			return fmt.Errorf("error receiving ACK for batch %d: %v", batchNumber, err)
		}

		// Verify ACK is for the correct batch
		if int(ackMessage.BatchNumber) != batchNumber {
			return fmt.Errorf("received ACK for batch %d but expected batch %d",
				ackMessage.BatchNumber, batchNumber)
		}
	}

	// Send notification that all bets have been sent
	if err := SendNotificationMessage(cs.connection, cs.agencyInfo.ID); err != nil {
		return fmt.Errorf("error sending notification message: %v", err)
	}

	// Receive notification response
	if err := ReceiveNotificationResponse(cs.connection); err != nil {
		return fmt.Errorf("error receiving notification response: %v", err)
	}

	log.Infof("action: communication_completed | result: success | client_id: %v | total_batches: %d",
		cs.agencyInfo.ID, cs.agencyInfo.batchCount)

	return nil
}

func (cs *AgencyService) ProcessWinnersQuery(signalChan <-chan os.Signal, attempt int, sleepTime time.Duration) error {
	select {
	case <-signalChan:
		log.Debugf("action: shutdown_signal_received | result: in_progress | client_id: %v | winners_query_attempt: %d",
			cs.agencyInfo.ID, attempt)
		return fmt.Errorf("client interrupted during winners query")
	default:
	}

	log.Infof("action: winners_query_attempt | result: in_progress | client_id: %v | attempt: %d",
		cs.agencyInfo.ID, attempt+1)

	// Send winners request
	if err := SendWinnersRequestMessage(cs.connection, cs.agencyInfo.ID); err != nil {
		log.Errorf("action: send_winners_request | result: fail | client_id: %v | attempt: %d | error: %v",
			cs.agencyInfo.ID, attempt+1, err)
		return fmt.Errorf("error sending winners request: %v", err)
	}

	// Receive winners response
	available, winners, err := ReceiveWinnersResponse(cs.connection)
	if err != nil {
		log.Errorf("action: receive_winners_response | result: fail | client_id: %v | attempt: %d | error: %v",
			cs.agencyInfo.ID, attempt+1, err)
		return fmt.Errorf("error receiving winners response: %v", err)
	}

	if available {
		// Winners are available!
		log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", len(winners))

		// Send close message only when winners are available
		if err := cs.SendCloseMessage(); err != nil {
			log.Errorf("action: send_close_message | result: fail | client_id: %v | error: %v",
				cs.agencyInfo.ID, err)
		}

		return nil
	}

	log.Debugf("action: winners_not_available | result: in_progress | client_id: %v | attempt: %d | sleep_time: %v",
		cs.agencyInfo.ID, attempt+1, sleepTime)

	return fmt.Errorf("winners not available yet")
}

func (cs *AgencyService) ReceiveAckMessage() (*AckBetMessage, error) {
	return ReceiveAckMessage(cs.connection)
}

func (cs *AgencyService) SendCloseMessage() error {
	return SendCloseMessage(cs.connection)
}

func (c *AgencyInfo) GetBatch(index int) ([]Bet, error) {
	if index < 0 || index >= c.batchCount {
		return nil, fmt.Errorf("batch index %d out of range [0, %d)", index, c.batchCount)
	}

	c.currentOffset = index * c.BatchMaxAmount

	return c.readNextBatch()
}

