package common

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

// Package common provides protocol definitions and message handling for the betting system.
//
// Protocol Overview:
// - All messages start with a message type byte
// - For batch bet messages: agency number, batch number, bets count, then bet data
// - Agency number is a single byte (1 character)
// - Fixed fields (DNI, Birthday) have predefined sizes and no length prefix
// - Variable fields (Name, Surname) have a length byte followed by content
// - Maximum variable field size is 255 bytes
// - BetNumber is a fixed 4-byte field stored as big endian 32-bit integer
//
// Message Types:
// - MessageTypeBet (1): Client sends bet information to server (now supports batches)
// - MessageTypeAck (2): Server acknowledges batch of bets sent (type + agency + batch number)
// - MessageTypeClose (3): Graceful connection shutdown
// - MessageTypeNotification (4): Client notifies completion of bet sending
// - MessageTypeWinnersRequest (5): Client requests winners
// - MessageTypeWinnersNotAvailable (6): Server responds winners not ready
// - MessageTypeWinnersAvailable (7): Server responds with winners
//
// Batch Bet Message Format:
// 1st byte: message type (MessageTypeBet)
// 2nd byte: agency number
// 3rd-6th bytes: batch number (4-byte big endian integer)
// 7th byte: number of bets (n)
// Following bytes: n times (name + surname + DNI + birthday + bet number)
//
// ACK Message Format:
// 1st byte: message type (MessageTypeAck)
// 2nd byte: agency number
// 3rd-6th bytes: batch number (4-byte big endian integer)

// Protocol message types
const (
	MessageTypeBet                 = 1
	MessageTypeAck                 = 2
	MessageTypeClose               = 3
	MessageTypeNotification        = 4 // Client notifies completion of bet sending
	MessageTypeNotificationAck     = 5 // Server acknowledges notification
	MessageTypeWinnersRequest      = 6 // Client requests winners
	MessageTypeWinnersNotAvailable = 7 // Server responds winners not ready
	MessageTypeWinnersAvailable    = 8 // Server responds with winners
)

// Common protocol field sizes
const (
	MessageTypeSize      = 1
	AgencyNumberSize     = 1
	BatchNumberSize      = 4
	BetsCountSize        = 1
	DNISize              = 8
	BirthdaySize         = 10
	BetNumberSize        = 4
	MaxVariableFieldSize = 255
)

// ACK message protocol constants
const (
	// ACK message structure: [type][agency][batch_number]
	AckSize = MessageTypeSize + AgencyNumberSize + BatchNumberSize

	// Byte offsets in ACK message (after message type byte)
	AckAgencyOffset      = MessageTypeSize
	AckBatchNumberOffset = MessageTypeSize + AgencyNumberSize
)

// AckBetMessage represents an acknowledgment message for a batch of bets
type AckBetMessage struct {
	AgencyNumber byte
	BatchNumber  uint32 // Changed to uint32 for 4-byte batch number
}

// Error message constants
const (
	ErrInsufficientData = "insufficient data"
	ErrInvalidBetNumber = "invalid bet number: cannot be empty"
	ErrEmptyBetNumber   = "invalid BetNumber field: cannot be empty"
)

// DeserializeAckBetMessage deserializes ACK bet message according to the protocol:
// 1st byte: message type (MessageTypeAck)
// 2nd byte: agency number
// 3rd-6th bytes: batch number (4-byte big endian integer)
func DeserializeAckBetMessage(data []byte) (*AckBetMessage, error) {
	if len(data) < AckSize {
		return nil, fmt.Errorf("data too short: need at least %d bytes", AckSize)
	}

	// Validate message type
	if err := validateMessageType(data, MessageTypeAck); err != nil {
		return nil, err
	}

	ackMsg := &AckBetMessage{}

	// Extract fields using offset constants
	ackMsg.AgencyNumber = data[AckAgencyOffset]

	// Extract 4-byte batch number (big endian)
	batchNumberBytes := data[AckBatchNumberOffset : AckBatchNumberOffset+BatchNumberSize]
	ackMsg.BatchNumber = binary.BigEndian.Uint32(batchNumberBytes)

	return ackMsg, nil
}

// CreateCloseMessage creates a close message to gracefully shutdown the connection
func CreateCloseMessage() []byte {
	return []byte{MessageTypeClose}
}

// ReceiveAckMessage handles the protocol for receiving ACK messages from the server
func ReceiveAckMessage(connection Connection) (*AckBetMessage, error) {

	data, err := connection.ReceiveExactBytes(AckSize)
	if err != nil {
		return nil, fmt.Errorf("error reading ACK data: %v", err)
	}

	// Deserialize the complete ACK message
	return DeserializeAckBetMessage(data)
}

// SendBetMessage handles the protocol for sending bet messages to the server
func SendBetMessage(connection Connection, message *Message) error {
	return connection.Send(message.Content)
}

// SendCloseMessage handles the protocol for sending close messages to the server
func SendCloseMessage(connection Connection) error {
	closeMessage := CreateCloseMessage()
	return connection.Send(closeMessage)
}

// SendBatchBetMessage handles the protocol for sending batch bet messages to the server
func SendBatchBetMessage(connection Connection, agencyNumber string, batchNumber int, bets []Bet) error {
	batchMessage, err := SerializeBatchBetMessage(agencyNumber, batchNumber, bets)
	if err != nil {
		return fmt.Errorf("error serializing batch bet message: %v", err)
	}
	return connection.Send(batchMessage)
}

// CreateNotificationMessage creates a notification message to inform server that all bets have been sent
func CreateNotificationMessage(agencyNumber string) []byte {
	agencyNum := extractAgencyNumber(agencyNumber)
	return []byte{MessageTypeNotification, agencyNum}
}

// CreateWinnersRequestMessage creates a request message to ask for winners
func CreateWinnersRequestMessage(agencyNumber string) []byte {
	agencyNum := extractAgencyNumber(agencyNumber)
	return []byte{MessageTypeWinnersRequest, agencyNum}
}

// SendNotificationMessage sends a notification to the server that all bets have been sent
func SendNotificationMessage(connection Connection, agencyNumber string) error {
	notificationMessage := CreateNotificationMessage(agencyNumber)
	return connection.Send(notificationMessage)
}

// SendWinnersRequestMessage sends a winners request to the server
func SendWinnersRequestMessage(connection Connection, agencyNumber string) error {
	winnersRequestMessage := CreateWinnersRequestMessage(agencyNumber)
	return connection.Send(winnersRequestMessage)
}

// ReceiveWinnersResponse receives the response to a winners request
func ReceiveWinnersResponse(connection Connection) (bool, []string, error) {
	// First read the message type
	messageTypeData, err := connection.ReceiveExactBytes(1)
	if err != nil {
		return false, nil, fmt.Errorf("error reading message type: %v", err)
	}

	messageType := messageTypeData[0]

	switch messageType {
	case MessageTypeWinnersNotAvailable:
		return false, nil, nil

	case MessageTypeWinnersAvailable:
		// Read number of winners
		numWinnersData, err := connection.ReceiveExactBytes(1)
		if err != nil {
			return false, nil, fmt.Errorf("error reading number of winners: %v", err)
		}
		numWinners := int(numWinnersData[0])

		// Read each winner DNI (8 bytes each)
		winners := make([]string, numWinners)
		for i := 0; i < numWinners; i++ {
			dniData, err := connection.ReceiveExactBytes(DNISize)
			if err != nil {
				return false, nil, fmt.Errorf("error reading winner DNI %d: %v", i, err)
			}
			// Trim whitespace from DNI
			winners[i] = string(dniData)
			winners[i] = winners[i][:len(winners[i])-countTrailingSpaces(winners[i])]
		}

		return true, winners, nil

	default:
		return false, nil, fmt.Errorf("unexpected message type: %d", messageType)
	}
}

// countTrailingSpaces counts trailing spaces in a string
func countTrailingSpaces(s string) int {
	count := 0
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ' ' {
			count++
		} else {
			break
		}
	}
	return count
}

// ReceiveNotificationResponse receives the response to a notification message
func ReceiveNotificationResponse(connection Connection) error {
	// Read notification ACK response
	data, err := connection.ReceiveExactBytes(1)
	if err != nil {
		return fmt.Errorf("error reading notification response: %v", err)
	}

	if data[0] != MessageTypeNotificationAck {
		return fmt.Errorf("unexpected response type: %d, expected %d", data[0], MessageTypeNotificationAck)
	}

	return nil
}

// serializeBetNumberField converts a bet number string to 4-byte big endian format
func serializeBetNumberField(result []byte, betNumber string) ([]byte, error) {
	// Convert string to integer
	betInt, err := strconv.ParseUint(betNumber, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid bet number: %s is not a valid integer", betNumber)
	}

	// Convert to 4-byte big endian
	betBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(betBytes, uint32(betInt))
	result = append(result, betBytes...)

	return result, nil
}

// Helper functions for common operations

// serializeVariableField adds a variable length field to the result
// Format: [length_byte][content]
func serializeVariableField(result []byte, content string) []byte {
	contentBytes := []byte(content)
	if len(contentBytes) > MaxVariableFieldSize {
		contentBytes = contentBytes[:MaxVariableFieldSize]
	}
	result = append(result, byte(len(contentBytes)))
	result = append(result, contentBytes...)
	return result
}

// serializeFixedField adds a fixed length field to the result and validates its size
func serializeFixedField(result []byte, content string, expectedSize int, fieldName string) ([]byte, error) {
	contentBytes := []byte(content)
	if len(contentBytes) != expectedSize {
		return nil, fmt.Errorf("invalid %s field: must be %d characters", fieldName, expectedSize)
	}
	result = append(result, contentBytes...)
	return result, nil
}

// validateMessageType checks if the first byte matches the expected message type
func validateMessageType(data []byte, expectedType byte) error {
	if len(data) == 0 {
		return errors.New("empty data")
	}
	if data[0] != expectedType {
		return fmt.Errorf("invalid message type: expected %d, got %d", expectedType, data[0])
	}
	return nil
}

// extractAgencyNumber safely extracts agency number from AgencyInfo ID
func extractAgencyNumber(id string) byte {
	if len(id) > 0 {
		if agencyNum, err := strconv.ParseUint(id, 10, 8); err == nil {
			return byte(agencyNum)
		}
	}
	return 0 // Default fallback
}

// SerializeBatchBetMessage converts a batch of bets to bytes according to the new protocol:
// 1st byte: message type (MessageTypeBet)
// 2nd byte: agency number
// 3rd byte: batch number
// 4th byte: number of bets (n)
// Following bytes: n times (name + surname + DNI + birthday + bet number)
func SerializeBatchBetMessage(agencyNumber string, batchNumber int, bets []Bet) ([]byte, error) {
	var result []byte
	var err error

	// 1st byte: message type
	result = append(result, MessageTypeBet)

	// 2nd byte: agency number
	agencyNum := extractAgencyNumber(agencyNumber)
	result = append(result, agencyNum)

	// 3rd-6th bytes: batch number (4-byte big endian integer)
	if batchNumber < 0 || batchNumber > 0xFFFFFFFF {
		return nil, fmt.Errorf("batch number out of range: %d (must be 0 to %d)", batchNumber, 0xFFFFFFFF)
	}
	batchBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(batchBytes, uint32(batchNumber))
	result = append(result, batchBytes...)

	// 7th byte: number of bets
	numBets := len(bets)
	if numBets > 255 {
		return nil, fmt.Errorf("too many bets in batch: %d (max 255)", numBets)
	}
	result = append(result, byte(numBets))

	// Serialize each bet
	for _, bet := range bets {
		// Serialize variable fields (name and surname)
		result = serializeVariableField(result, bet.Name)
		result = serializeVariableField(result, bet.Surname)

		// Serialize fixed fields (DNI and birthday)
		result, err = serializeFixedField(result, bet.DNI, DNISize, "DNI")
		if err != nil {
			return nil, err
		}

		result, err = serializeFixedField(result, bet.Birthday, BirthdaySize, "Birthday")
		if err != nil {
			return nil, err
		}

		// Serialize BetNumber field (4-byte big endian integer)
		if len(bet.BetNumber) == 0 {
			return nil, fmt.Errorf("invalid BetNumber field: cannot be empty")
		}
		result, err = serializeBetNumberField(result, bet.BetNumber)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
