package common

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Package common provides protocol definitions and message handling for the betting system.
//
// Protocol Overview:
// - All messages start with a message type byte
// - Agency number is a single byte (1 character)
// - Fixed fields (DNI, Birthday) have predefined sizes and no length prefix
// - Variable fields (Name, Surname) have a length byte followed by content
// - Maximum variable field size is 255 bytes
// - BetNumber is a fixed 4-byte field stored as big endian 32-bit integer
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
	MessageTypeSize      = 1
	AgencyNumberSize     = 1
	DNISize              = 8
	BirthdaySize         = 10
	BetNumberSize        = 4
	MaxVariableFieldSize = 255
)

// ACK message protocol constants
const (
	// ACK message structure: [type][agency][dni][bet_number]
	AckSize = MessageTypeSize + AgencyNumberSize + DNISize + BetNumberSize

	// Byte offsets in ACK message (after message type byte)
	AckAgencyOffset    = MessageTypeSize
	AckDNIOffset       = MessageTypeSize + AgencyNumberSize
	AckBetNumberOffset = MessageTypeSize + AgencyNumberSize + DNISize
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
	ErrInvalidBetNumber = "invalid bet number: cannot be empty"
	ErrEmptyBetNumber   = "invalid BetNumber field: cannot be empty"
)

// SerializeBetMessage converts AgencyInfo to bytes according to the protocol:
// 1st byte: message type (MessageTypeBet)
// 2nd byte: agency number (same as ID)
// Fixed length fields (DNI=8 chars, Birthday=10 chars): direct content
// Variable length fields (Name, Surname): 1 byte length + string content
// BetNumber: 4-byte big endian integer
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

	// Serialize BetNumber field (4-byte big endian integer)
	if len(agencyInfo.BetNumber) == 0 {
		return nil, errors.New(ErrEmptyBetNumber)
	}
	result, err = serializeBetNumberField(result, agencyInfo.BetNumber)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// DeserializeAckBetMessage deserializes ACK bet message according to the protocol:
// 1st byte: message type (MessageTypeAck)
// 2nd byte: agency number
// Following 8 bytes: DNI
// Following 4 bytes: bet number (big endian 32-bit integer)
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

	// Extract and clean DNI
	dniStart := AckDNIOffset
	dniBytes := data[dniStart : dniStart+DNISize]
	ackMsg.DNI = strings.TrimRight(string(dniBytes), " ")

	// Extract bet number (4-byte big endian integer)
	betNumberStart := AckBetNumberOffset
	betNumberBytes := data[betNumberStart : betNumberStart+BetNumberSize]
	ackMsg.BetNumber = deserializeBetNumberField(betNumberBytes)

	// Validate bet number is not empty
	if len(ackMsg.BetNumber) == 0 {
		return nil, errors.New(ErrInvalidBetNumber)
	}

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

// deserializeBetNumberField converts 4-byte big endian format to bet number string
func deserializeBetNumberField(betBytes []byte) string {
	if len(betBytes) != 4 {
		return ""
	}

	// Convert from 4-byte big endian to integer
	betInt := binary.BigEndian.Uint32(betBytes)

	// Convert to string
	return strconv.FormatUint(uint64(betInt), 10)
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
