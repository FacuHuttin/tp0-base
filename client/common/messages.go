package common

import (
	"encoding/binary"
	"fmt"
	"strconv"
)

func encode_bet_message(b *Bet, agencyId string) ([]uint8, error) {
	var data []uint8

	// Convert Agency ID to byte and encode it
	agencyIdNum, err := strconv.Atoi(agencyId)
	if err != nil || agencyIdNum < 0 || agencyIdNum > 255 {
		return nil, fmt.Errorf("invalid agencyId: %v", agencyId)
	}
	data = append(data, uint8(agencyIdNum))

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
	if len(data) < 1 {
		return fmt.Errorf("data is too short to contain a confirmation message")
	}

	// Read the confirmation message (uint8)
	confirmationMessage := data[0]

	// Check the value of the confirmation message
	if confirmationMessage == 1 {
		return nil
	} else {
		return fmt.Errorf("confirmation message is not 1, got %d", confirmationMessage)
	}
}