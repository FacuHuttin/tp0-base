package common

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

// Package common provides protocol definitions and message handling for the betting system client.
//
// This package handles message serialization, deserialization, and protocol constants
// for communication between client and server in the betting system.
//
// Protocol Overview:
// - All messages start with a message type byte (1 byte)
// - Supports batch processing for efficiency and performance
// - Fixed fields (DNI, Birthday) have predefined sizes and no length prefix
// - Variable fields (Name, Surname) have a length byte followed by content
// - BetNumber is a fixed 4-byte field stored as big endian 32-bit integer
// - Maximum variable field size is 255 bytes
//
// Message Types:
// - MessageTypeBet (1): Client sends bet information to server (supports batches)
// - MessageTypeAck (2): Server acknowledges batch receipt
// - MessageTypeClose (3): Graceful connection shutdown
// - MessageTypeNotification (4): Client notifies completion of bet sending
// - MessageTypeNotificationAck (5): Server acknowledges notification
// - MessageTypeWinnersRequest (6): Client requests winners
// - MessageTypeWinnersNotAvailable (7): Server responds winners not ready
// - MessageTypeWinnersAvailable (8): Server responds with winners
//
// Batch Bet Message Format:
// 1st byte: message type (MessageTypeBet)
// 2nd byte: agency number (0-255)
// 3rd-6th bytes: batch number (4-byte big endian integer)
// 7th byte: number of bets in batch (n, max 255)
// Following bytes: n bet records, each containing:
//   - Name: [length_byte][utf-8_content]
//   - Surname: [length_byte][utf-8_content]
//   - DNI: [8_fixed_bytes]
//   - Birthday: [10_fixed_bytes]
//   - BetNumber: [4_bytes_big_endian_integer]
//
// ACK Message Format:
// 1st byte: message type (MessageTypeAck)
// 2nd byte: agency number (0-255)
// 3rd-6th bytes: batch number (4-byte big endian integer)
//
// Winners Response Format:
// 1st byte: MessageTypeWinnersAvailable
// 2nd byte: number of winners (0-255)
// Following bytes: winner DNIs (8 bytes each, fixed length, space-padded)

// Protocol message types
const (
	MessageTypeBet                 = 1
	MessageTypeAck                 = 2
	MessageTypeClose               = 3
	MessageTypeNotification        = 4
	MessageTypeNotificationAck     = 5
	MessageTypeWinnersRequest      = 6
	MessageTypeWinnersNotAvailable = 7
	MessageTypeWinnersAvailable    = 8
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
	WinnersCountSize     = 1
	MaxVariableFieldSize = 255
	Max4ByteInt          = 0xFFFFFFFF
)

// ACK message protocol constants
const (
	// ACK message structure: [type][agency][batch_number]
	AckSize = MessageTypeSize + AgencyNumberSize + BatchNumberSize

	// Byte offsets in ACK message (after message type byte)
	AckAgencyOffset      = MessageTypeSize
	AckBatchNumberOffset = MessageTypeSize + AgencyNumberSize
)

// AckBetMessage represents an acknowledgment message for a batch of bets.
//
// Used to confirm successful receipt of a batch of bets from a specific agency.
// Contains the agency number and batch number for identification and tracking.
//
// Fields:
//   - AgencyNumber: The agency that sent the batch (0-255)
//   - BatchNumber: The batch identifier (0-4294967295)
type AckBetMessage struct {
	AgencyNumber byte
	BatchNumber  uint32 // 4-byte batch number for large batch support
}

// Error message constants
const (
	ErrInsufficientData = "insufficient data"
	ErrInvalidBetNumber = "invalid bet number: cannot be empty"
	ErrEmptyBetNumber   = "invalid BetNumber field: cannot be empty"
)

// DeserializeAckBetMessage deserializes ACK bet message according to the protocol.
//
// Protocol format:
//   - 1st byte: message type (MessageTypeAck)
//   - 2nd byte: agency number (0-255)
//   - 3rd-6th bytes: batch number (4-byte big endian integer)
//
// Parameters:
//   - data: Raw message bytes to deserialize (must be at least 6 bytes)
//
// Returns:
//   - *AckBetMessage: Parsed acknowledgment message
//   - error: Error if data is malformed, too short, or has invalid message type
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

// CreateCloseMessage creates a close message to gracefully shutdown the connection.
//
// Returns:
//   - []byte: Single-byte close message (MessageTypeClose)
func CreateCloseMessage() []byte {
	return []byte{MessageTypeClose}
}

// ReceiveAckMessage handles the protocol for receiving ACK messages from the server.
//
// Reads exactly the required number of bytes for an ACK message and deserializes it.
//
// Parameters:
//   - connection: Active connection to read from
//
// Returns:
//   - *AckBetMessage: Parsed acknowledgment message
//   - error: Error if connection fails or message is malformed
func ReceiveAckMessage(connection Connection) (*AckBetMessage, error) {

	data, err := connection.ReceiveExactBytes(AckSize)
	if err != nil {
		return nil, fmt.Errorf("error reading ACK data: %v", err)
	}

	// Deserialize the complete ACK message
	return DeserializeAckBetMessage(data)
}

// SendBetMessage handles the protocol for sending bet messages to the server.
//
// Sends the pre-serialized message content over the connection.
//
// Parameters:
//   - connection: Active connection to send through
//   - message: Message containing serialized bet data
//
// Returns:
//   - error: Error if sending fails
func SendBetMessage(connection Connection, message *Message) error {
	return connection.Send(message.Content)
}

// SendCloseMessage handles the protocol for sending close messages to the server.
//
// Creates and sends a graceful shutdown message to properly close the connection.
//
// Parameters:
//   - connection: Active connection to send through
//
// Returns:
//   - error: Error if sending fails
func SendCloseMessage(connection Connection) error {
	closeMessage := CreateCloseMessage()
	return connection.Send(closeMessage)
}

// SendBatchBetMessage handles the protocol for sending batch bet messages to the server.
//
// Serializes a batch of bets and sends them as a single message for efficiency.
//
// Parameters:
//   - connection: Active connection to send through
//   - agencyNumber: Agency identifier string (converted to byte)
//   - batchNumber: Batch identifier (0-4294967295)
//   - bets: Slice of bet objects to send (max 255 bets per batch)
//
// Returns:
//   - error: Error if serialization fails or sending fails
func SendBatchBetMessage(connection Connection, agencyNumber string, batchNumber int, bets []Bet) error {
	batchMessage, err := SerializeBatchBetMessage(agencyNumber, batchNumber, bets)
	if err != nil {
		return fmt.Errorf("error serializing batch bet message: %v", err)
	}
	return connection.Send(batchMessage)
}

// CreateNotificationMessage creates a notification message to inform server that all bets have been sent.
//
// Protocol format: [MessageTypeNotification][agency_number]
//
// Parameters:
//   - agencyNumber: Agency identifier string (converted to byte)
//
// Returns:
//   - []byte: 2-byte notification message
func CreateNotificationMessage(agencyNumber string) []byte {
	agencyNum := extractAgencyNumber(agencyNumber)
	return []byte{MessageTypeNotification, agencyNum}
}

// CreateWinnersRequestMessage creates a request message to ask for winners.
//
// Protocol format: [MessageTypeWinnersRequest][agency_number]
//
// Parameters:
//   - agencyNumber: Agency identifier string (converted to byte)
//
// Returns:
//   - []byte: 2-byte winners request message
func CreateWinnersRequestMessage(agencyNumber string) []byte {
	agencyNum := extractAgencyNumber(agencyNumber)
	return []byte{MessageTypeWinnersRequest, agencyNum}
}

// SendNotificationMessage sends a notification to the server that all bets have been sent.
//
// Creates and sends a completion notification for the specified agency.
//
// Parameters:
//   - connection: Active connection to send through
//   - agencyNumber: Agency identifier string
//
// Returns:
//   - error: Error if sending fails
func SendNotificationMessage(connection Connection, agencyNumber string) error {
	notificationMessage := CreateNotificationMessage(agencyNumber)
	return connection.Send(notificationMessage)
}

// SendWinnersRequestMessage sends a winners request to the server.
//
// Creates and sends a request for winners from the specified agency.
//
// Parameters:
//   - connection: Active connection to send through
//   - agencyNumber: Agency identifier string
//
// Returns:
//   - error: Error if sending fails
func SendWinnersRequestMessage(connection Connection, agencyNumber string) error {
	winnersRequestMessage := CreateWinnersRequestMessage(agencyNumber)
	return connection.Send(winnersRequestMessage)
}

// ReceiveWinnersResponse receives the response to a winners request.
//
// Handles both "winners available" and "winners not available" responses from the server.
//
// Protocol formats:
//   - Not available: [MessageTypeWinnersNotAvailable]
//   - Available: [MessageTypeWinnersAvailable][num_winners][dni1][dni2]...[dniN]
//     Each DNI is 8 bytes fixed length, space-padded
//
// Parameters:
//   - connection: Active connection to read from
//
// Returns:
//   - bool: true if winners are available, false if not ready
//   - []string: List of winning DNIs (empty if not available)
//   - error: Error if connection fails or unexpected message type
func ReceiveWinnersResponse(connection Connection) (bool, []string, error) {
	// First read the message type
	messageTypeData, err := connection.ReceiveExactBytes(MessageTypeSize)
	if err != nil {
		return false, nil, fmt.Errorf("error reading message type: %v", err)
	}

	messageType := messageTypeData[0]

	switch messageType {
	case MessageTypeWinnersNotAvailable:
		return false, nil, nil

	case MessageTypeWinnersAvailable:
		// Read number of winners
		numWinnersData, err := connection.ReceiveExactBytes(WinnersCountSize)
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

// countTrailingSpaces counts trailing spaces in a string.
//
// Used to trim space-padding from fixed-length DNI fields.
//
// Parameters:
//   - s: String to analyze
//
// Returns:
//   - int: Number of trailing space characters
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

// ReceiveNotificationResponse receives the response to a notification message.
//
// Waits for and validates the server's acknowledgment of the completion notification.
//
// Parameters:
//   - connection: Active connection to read from
//
// Returns:
//   - error: Error if connection fails or unexpected response type
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

// serializeBetNumberField converts a bet number string to 4-byte big endian format.
//
// Parameters:
//   - result: Byte slice to append to
//   - betNumber: Bet number as string (must be valid 32-bit unsigned integer)
//
// Returns:
//   - []byte: Updated byte slice with bet number appended
//   - error: Error if bet number is invalid or out of 32-bit range
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

// serializeVariableField adds a variable length field to the result.
//
// Protocol format: [length_byte][content]
//
// Parameters:
//   - result: Byte slice to append to
//   - content: String content to serialize
//
// Returns:
//   - []byte: Updated byte slice with length-prefixed field appended
//
// Note: Content longer than 255 bytes will be truncated.
func serializeVariableField(result []byte, content string) []byte {
	contentBytes := []byte(content)
	if len(contentBytes) > MaxVariableFieldSize {
		contentBytes = contentBytes[:MaxVariableFieldSize]
	}
	result = append(result, byte(len(contentBytes)))
	result = append(result, contentBytes...)
	return result
}

// serializeFixedField adds a fixed length field to the result and validates its size.
//
// Parameters:
//   - result: Byte slice to append to
//   - content: String content to serialize
//   - expectedSize: Expected byte length after encoding
//   - fieldName: Field name for error messages
//
// Returns:
//   - []byte: Updated byte slice with fixed field appended
//   - error: Error if content size doesn't match expected size
func serializeFixedField(result []byte, content string, expectedSize int, fieldName string) ([]byte, error) {
	contentBytes := []byte(content)
	if len(contentBytes) != expectedSize {
		return nil, fmt.Errorf("invalid %s field: must be %d characters", fieldName, expectedSize)
	}
	result = append(result, contentBytes...)
	return result, nil
}

// validateMessageType checks if the first byte matches the expected message type.
//
// Parameters:
//   - data: Message bytes to validate
//   - expectedType: Expected message type value
//
// Returns:
//   - error: Error if data is empty or message type doesn't match
func validateMessageType(data []byte, expectedType byte) error {
	if len(data) == 0 {
		return errors.New("empty data")
	}
	if data[0] != expectedType {
		return fmt.Errorf("invalid message type: expected %d, got %d", expectedType, data[0])
	}
	return nil
}

// extractAgencyNumber safely extracts agency number from AgencyInfo ID.
//
// Converts string agency ID to byte value with fallback to 0 for invalid input.
//
// Parameters:
//   - id: Agency identifier string
//
// Returns:
//   - byte: Agency number (0-255), defaults to 0 if invalid
func extractAgencyNumber(id string) byte {
	if len(id) > 0 {
		if agencyNum, err := strconv.ParseUint(id, 10, 8); err == nil {
			return byte(agencyNum)
		}
	}
	return 0 // Default fallback
}

// SerializeBatchBetMessage converts a batch of bets to bytes according to the protocol.
//
// Protocol format:
//   - 1st byte: message type (MessageTypeBet)
//   - 2nd byte: agency number (0-255)
//   - 3rd-6th bytes: batch number (4-byte big endian integer)
//   - 7th byte: number of bets (n, max 255)
//   - Following bytes: n bet records, each containing:
//   - Name: [length_byte][content]
//   - Surname: [length_byte][content]
//   - DNI: [8_fixed_bytes]
//   - Birthday: [10_fixed_bytes]
//   - BetNumber: [4_bytes_big_endian]
//
// Parameters:
//   - agencyNumber: Agency identifier string (converted to byte)
//   - batchNumber: Batch identifier (0-4294967295)
//   - bets: Slice of bet objects to serialize (max 255 bets)
//
// Returns:
//   - []byte: Serialized batch message
//   - error: Error if parameters are invalid or serialization fails
func SerializeBatchBetMessage(agencyNumber string, batchNumber int, bets []Bet) ([]byte, error) {
	var result []byte
	var err error

	// 1st byte: message type
	result = append(result, MessageTypeBet)

	// 2nd byte: agency number
	agencyNum := extractAgencyNumber(agencyNumber)
	result = append(result, agencyNum)

	// 3rd-6th bytes: batch number (4-byte big endian integer)
	if batchNumber < 0 || batchNumber > Max4ByteInt {
		return nil, fmt.Errorf("batch number out of range: %d (must be 0 to %d)", batchNumber, Max4ByteInt)
	}
	batchBytes := make([]byte, BatchNumberSize)
	binary.BigEndian.PutUint32(batchBytes, uint32(batchNumber))
	result = append(result, batchBytes...)

	// 7th byte: number of bets
	numBets := len(bets)
	if numBets > MaxVariableFieldSize {
		return nil, fmt.Errorf("too many bets in batch: %d (max %d)", numBets, MaxVariableFieldSize)
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
