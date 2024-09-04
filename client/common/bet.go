package common

import (
	"strconv"
)

// Bet sent to server from client
type Bet struct {
	Name       string
	Surname    string
	DNI        uint32
	Birthday   string
	LotteryNum uint16
}

func NewBet(name string, surname string, dni string, birthday string, lotterynum string) *Bet {
	dniUint, err := strconv.ParseUint(dni, 10, 32)
	if err != nil {
		log.Criticalf("Invalid dni: %v",err)
	}
	lotteryNumUint, err := strconv.ParseUint(lotterynum, 10, 16)
	if err != nil {
		log.Criticalf("Invalid lottery number: %v", err)
	}
	bet := &Bet{
		Name:       name,
		Surname:    surname,
		DNI:        uint32(dniUint),
		Birthday:   birthday,
		LotteryNum: uint16(lotteryNumUint),
	}
	return bet
}
