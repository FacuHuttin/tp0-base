package common

import (
	"errors"
	"fmt"
	"strings"
)

// Package common provides protocol definitions and message handling for the betting system.
//
// Protocol Overview:
// - All messages start with a message type byte
// - Fixed fields (DNI, Birthday) have predefined sizes and no length prefix
// - Variable fields (Name, Surname, BetNumber) have a length byte followed by content
// - Maximum variable field size is 255 bytes
//
// Message Types:
// - MessageTypeBet (1): Client sends bet information to server
// - MessageTypeAck (2): Server acknowledges bet sent
// - MessageTypeClose (3): Graceful connection shutdown

// Protocol message types
const (
	MessageTypeBet   = 1
	MessageTypeAck   = 2
	MessageTypeClose = 3
)

// Common protocol field sizes
const (
	AgencyNumberSize     = 1
	DNISize              = 8
	BirthdaySize         = 10
	MaxVariableFieldSize = 255
)

// ACK message protocol constants
const (
	// ACK message structure: [type][agency][dni][bet_len][bet_number]
	AckMinimumSize = AgencyNumberSize + DNISize + 1 // agency + dni + bet_number_length

	// Byte offsets in ACK message (after message type byte)
	AckAgencyOffset     = 0
	AckDNIOffset        = AgencyNumberSize
	AckBetLengthOffset  = AgencyNumberSize + DNISize
	AckBetContentOffset = AckMinimumSize
)

// AckBetMessage represents an acknowledgment message for a bet
type AckBetMessage struct {
	AgencyNumber byte
	DNI          string
	BetNumber    string
}

// Error message constants
const (
	ErrInsufficientData = "insufficient data"
	ErrInvalidBetNumber = "invalid bet number: length cannot be 0"
	ErrEmptyBetNumber   = "invalid BetNumber field: cannot be empty"
)

// SerializeBetMessage converts AgencyInfo to bytes according to the protocol:
// 1st byte: message type (MessageTypeBet)
// 2nd byte: agency number (same as ID)
// Fixed length fields (DNI=8 chars, Birthday=10 chars): direct content
// Variable length fields (Name, Surname, BetNumber): 1 byte length + string content
func SerializeBetMessage(agencyInfo *AgencyInfo) ([]byte, error) {
	var result []byte
	var err error

	// Add message type and agency number
	result = append(result, MessageTypeBet)
	result = append(result, extractAgencyNumber(agencyInfo.ID))

	// Serialize variable fields
	result = serializeVariableField(result, agencyInfo.Name)
	result = serializeVariableField(result, agencyInfo.Surname)

	// Serialize fixed fields
	result, err = serializeFixedField(result, agencyInfo.DNI, DNISize, "DNI")
	if err != nil {
		return nil, err
	}

	result, err = serializeFixedField(result, agencyInfo.Birthday, BirthdaySize, "Birthday")
	if err != nil {
		return nil, err
	}

	// Serialize BetNumber field (variable, but cannot be empty)
	if len(agencyInfo.BetNumber) == 0 {
		return nil, errors.New(ErrEmptyBetNumber)
	}
	result = serializeVariableField(result, agencyInfo.BetNumber)

	return result, nil
}

// DeserializeAckBetMessage deserializes ACK bet message according to the protocol:
// 1st byte: message type (MessageTypeAck)
// 2nd byte: agency number
// Following 8 bytes: DNI
// 1 byte: bet number length
// Following bytes: bet number content
func DeserializeAckBetMessage(data []byte) (*AckBetMessage, error) {
	minSizeWithType := 1 + AckMinimumSize
	if len(data) < minSizeWithType {
		return nil, fmt.Errorf("data too short: need at least %d bytes", minSizeWithType)
	}

	// Validate message type
	if err := validateMessageType(data, MessageTypeAck); err != nil {
		return nil, err
	}

	ackMsg := &AckBetMessage{}

	// Extract fields using offset constants
	ackMsg.AgencyNumber = data[1+AckAgencyOffset]

	// Extract and clean DNI
	dniStart := 1 + AckDNIOffset
	dniBytes := data[dniStart : dniStart+DNISize]
	ackMsg.DNI = strings.TrimRight(string(dniBytes), " ")

	// Extract bet number length and validate
	betLengthOffset := 1 + AckBetLengthOffset
	betNumberLength := int(data[betLengthOffset])
	if betNumberLength == 0 {
		return nil, errors.New(ErrInvalidBetNumber)
	}

	// Validate we have enough data for bet number content
	betContentOffset := 1 + AckBetContentOffset
	if len(data) < betContentOffset+betNumberLength {
		return nil, fmt.Errorf("data too short: need %d bytes for bet number", betNumberLength)
	}

	// Extract bet number content
	ackMsg.BetNumber = string(data[betContentOffset : betContentOffset+betNumberLength])

	return ackMsg, nil
}

// GetMinimumBytes returns the minimum number of bytes needed for an ACK message
func (ack *AckBetMessage) GetMinimumBytes() int {
	return 1 + AckMinimumSize // 1 for message type + original minimum size
}

// GetTotalBytesFromData calculates the total bytes needed based on the initial data
func GetTotalBytesFromMinData(minData []byte) (int, error) {
	minSizeWithType := 1 + AckMinimumSize
	if len(minData) < minSizeWithType {
		return 0, errors.New(ErrInsufficientData)
	}

	// Extract bet number length from the specified offset (accounting for message type byte)
	betLengthOffset := 1 + AckBetLengthOffset
	betNumberLength := int(minData[betLengthOffset])

	// Total = minimum bytes with type + bet number content length
	return minSizeWithType + betNumberLength, nil
}

// GetBetNumberLengthFromData extracts the bet number length from minimum data
func GetBetNumberLengthFromMinData(minData []byte) (int, error) {
	minSizeWithType := 1 + AckMinimumSize
	if len(minData) < minSizeWithType {
		return 0, errors.New(ErrInsufficientData)
	}

	// Extract bet number length from the correct offset (accounting for message type byte)
	betLengthOffset := 1 + AckBetLengthOffset
	betNumberLength := int(minData[betLengthOffset])
	if betNumberLength == 0 {
		return 0, errors.New(ErrInvalidBetNumber)
	}

	return betNumberLength, nil
}

// CreateCloseMessage creates a close message to gracefully shutdown the connection
func CreateCloseMessage() []byte {
	return []byte{MessageTypeClose}
}

// ReceiveAckMessage handles the protocol for receiving ACK messages from the server
func ReceiveAckMessage(connection Connection) (*AckBetMessage, error) {
	// Create a temporary AckBetMessage to get protocol information
	tempAck := &AckBetMessage{}
	minBytes := tempAck.GetMinimumBytes()

	// Read the minimum required bytes
	minData, err := connection.ReceiveExactBytes(minBytes)
	if err != nil {
		return nil, fmt.Errorf("error reading initial ACK data: %v", err)
	}

	// Get bet number length from the data
	betNumberLength, err := GetBetNumberLengthFromMinData(minData)
	if err != nil {
		return nil, fmt.Errorf("error extracting bet number length: %v", err)
	}

	// Read the bet number content (bet number length is guaranteed to be > 0)
	betData, err := connection.ReceiveExactBytes(betNumberLength)
	if err != nil {
		return nil, fmt.Errorf("error reading bet number data: %v", err)
	}

	// Combine all data
	totalBytes, err := GetTotalBytesFromMinData(minData)
	if err != nil {
		return nil, fmt.Errorf("error calculating total bytes: %v", err)
	}

	fullData := make([]byte, totalBytes)
	copy(fullData, minData)
	copy(fullData[minBytes:], betData)

	// Deserialize the complete ACK message
	return DeserializeAckBetMessage(fullData)
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
		return id[0]
	}
	return '0' // Default fallback
}
