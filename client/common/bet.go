package common

import (
	"strconv"
	"os"
	"bufio"
	"fmt"
	"strings"
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

func GetBetsFromCsv(csvFileName string) ([]Bet, error) {
	var bets []Bet

	// Open the CSV file
	csvFile, err := os.Open(csvFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %v", err)
	}
	defer csvFile.Close()

	// Read the CSV file
	scanner := bufio.NewScanner(csvFile)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}
		bet := NewBet(fields[0], fields[1], fields[2], fields[3], fields[4])
		bets = append(bets, *bet)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %v", err)
	}

	return bets, nil
}
