package common

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"os"
)

func encode_batch_message(bets []Bet, clientID string, lastBetsBatch bool) ([]byte, error) {
	var data []byte

	// Add the message id number
	msgId := uint8(BET_BATCH_MESSAGE_ID)
	data = append(data, msgId)

	// Convert Agency ID to byte and encode it
	agencyIdNum, err := strconv.Atoi(clientID)
	if err != nil || agencyIdNum < 0 || agencyIdNum > 255 {
		return nil, fmt.Errorf("invalid agencyId: %v", clientID)
	}
	data = append(data, uint8(agencyIdNum))
	
	// Add the last batch flag
	if lastBetsBatch {
		data = append(data, 1)
	} else {
		data = append(data, 0)
	}

	// Add the number of bets
	numBets := uint8(len(bets))
	data = append(data, numBets)

	// Encode each bet and append to data
	for _, bet := range bets {
		encodedBet, err := encode_bet_message(&bet)
		if err != nil {
			return nil, fmt.Errorf("failed to encode bet: %v", err)
		}
		data = append(data, encodedBet...)
	}

	return data, nil
}

func encode_bet_message(b *Bet) ([]uint8, error) {
	var data []uint8

	// Encode Name
	nameLen := uint8(len(b.Name))
	data = append(data, nameLen)
	data = append(data, []uint8(b.Name)...)

	// Encode Surname
	surnameLen := uint8(len(b.Surname))
	data = append(data, surnameLen)
	data = append(data, []uint8(b.Surname)...)

	// Encode DNI (uint32) in Big Endian
	dniNumBytes := make([]uint8, 4)
	binary.BigEndian.PutUint32(dniNumBytes, b.DNI)
	data = append(data, dniNumBytes...)

	// Encode Birthday (10 bytes)
	if len(b.Birthday) != 10 {
		return nil, fmt.Errorf("birthday must be 10 characters long")
	}
	data = append(data, []uint8(b.Birthday)...)

	// Encode LotteryNum (uint16) in Big Endian
	lotteryNumBytes := make([]uint8, 2)
	binary.BigEndian.PutUint16(lotteryNumBytes, b.LotteryNum)
	data = append(data, lotteryNumBytes...)

	return data, nil
}

func decode_confirmation_message(data []byte) error {

	// Read the confirmation message (uint8)
	confirmationMessage := data[0]

	// Check the value of the confirmation message
	if confirmationMessage == ACK_MESSAGE_ID {
		return nil
	} else if confirmationMessage == ERROR_MESSAGE_ID {
		return fmt.Errorf("server returned error from confirmation message")
	} else {
		return fmt.Errorf("invalid confirmation message: %d", confirmationMessage)
	}
}

func decode_winners_message(serverSocket *TCPConn, sigChannel chan os.Signal) (int, error) {
	buffer := make([]byte, 1)

	// Read the ID of the winners message
	select {
	case <-sigChannel:
		return 0, fmt.Errorf("operation interrupted by SIGTERM")
	default:
	_, err := serverSocket.ReadFull(buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read winners message: %v", err)
		}
	}

	id_winner_message := int(buffer[0])
	if id_winner_message != WINNERS_MESSAGE_ID {
		return 0, fmt.Errorf("invalid winners message id: %d", id_winner_message)
	}

	// Read the number of winners
	select {
	case <-sigChannel:
		return 0, fmt.Errorf("operation interrupted by SIGTERM")
	default:
		_, err := serverSocket.ReadFull(buffer)
		if err != nil {
			return 0, fmt.Errorf("failed to read number of winners: %v", err)
		}
	}

	winners_amount := int(buffer[0])

	for i := 0; i < winners_amount; i++ {
		// Read the winner ID
		dni_buffer := make([]byte, 4)
		select {
		case <-sigChannel:
			return 0, fmt.Errorf("operation interrupted by signal")
		default:
			_, err := serverSocket.ReadFull(dni_buffer)
			if err != nil {
				return 0, fmt.Errorf("failed to read winner dni: %v", err)
			}
		}
		winner_dni := binary.BigEndian.Uint32(dni_buffer)
		log.Debugf("Winner DNI: %d", winner_dni)
	}

	return winners_amount, nil
}