package common

import (
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
	Bets           [][]Bet
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
		Bets:           [][]Bet{},
	}

	if err := agency.LoadBetsFromFile(); err != nil {
		return nil, fmt.Errorf("error loading bets from file: %v", err)
	}

	return agency, nil
}

func (c *AgencyInfo) LoadBetsFromFile() error {
	file, err := os.Open(c.BetsFile)
	if err != nil {
		return fmt.Errorf("error opening bets file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("error reading CSV file: %v", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV file is empty")
	}

	// Parse records into Bet structures
	var allBets []Bet
	for i, record := range records {
		if len(record) != 5 {
			return fmt.Errorf("invalid CSV format at row %d: expected 5 columns (Name,Surname,DNI,Birthday,BetNumber), got %d", i+1, len(record))
		}

		// Validate required fields are not empty
		for j, field := range record {
			if field == "" {
				fieldNames := []string{"Name", "Surname", "DNI", "Birthday", "BetNumber"}
				return fmt.Errorf("invalid CSV format at row %d: %s cannot be empty", i+1, fieldNames[j])
			}
		}

		bet := Bet{
			Name:      record[0],
			Surname:   record[1],
			DNI:       record[2],
			Birthday:  record[3],
			BetNumber: record[4],
		}
		allBets = append(allBets, bet)
	}

	// Create batches based on BatchMaxAmount
	var batches [][]Bet
	for i := 0; i < len(allBets); i += c.BatchMaxAmount {
		end := i + c.BatchMaxAmount
		if end > len(allBets) {
			end = len(allBets)
		}
		batch := allBets[i:end]
		batches = append(batches, batch)
	}

	c.Bets = batches

	fmt.Printf("Successfully loaded %d bets into %d batches (max %d bets per batch)\n",
		len(allBets), len(batches), c.BatchMaxAmount)

	return nil
}

// AgencyService handles the communication logic for the agency
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
	for batchIndex := 0; batchIndex < cs.agencyInfo.GetBatchCount(); batchIndex++ {
		// Check for signal before each batch
		select {
		case <-signalChan:
			log.Infof("action: signal_received | result: stopping | client_id: %v | batch: %d",
				cs.agencyInfo.ID, batchIndex)
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

	// Send close message after all batches are sent and acknowledged
	if err := cs.SendCloseMessage(); err != nil {
		return fmt.Errorf("error sending close message: %v", err)
	}

	log.Infof("action: communication_completed | result: success | client_id: %v | total_batches: %d",
		cs.agencyInfo.ID, cs.agencyInfo.GetBatchCount())

	return nil
}

func (cs *AgencyService) ReceiveAckMessage() (*AckBetMessage, error) {
	return ReceiveAckMessage(cs.connection)
}

func (cs *AgencyService) SendCloseMessage() error {
	return SendCloseMessage(cs.connection)
}

// GetBatch returns the batch at the specified index
func (c *AgencyInfo) GetBatch(index int) ([]Bet, error) {
	if index < 0 || index >= len(c.Bets) {
		return nil, fmt.Errorf("batch index %d out of range [0, %d)", index, len(c.Bets))
	}
	return c.Bets[index], nil
}

// GetBatchCount returns the total number of batches
func (c *AgencyInfo) GetBatchCount() int {
	return len(c.Bets)
}

// GetTotalBetsCount returns the total number of bets across all batches
func (c *AgencyInfo) GetTotalBetsCount() int {
	total := 0
	for _, batch := range c.Bets {
		total += len(batch)
	}
	return total
}
