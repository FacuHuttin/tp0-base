package common

// LotteryAgency
type LotteryAgency struct {
	client *Client
	bets   []Bet
}

// NewAgency Initializes a new LotteryAgency receiving the configuration
// as a parameter
func NewAgency(config ClientConfig, bets []Bet) *LotteryAgency {
	client := &Client{
		config: config,
	}
	lottery := &LotteryAgency{
		client: client,
		bets:   bets,
	}
	return lottery
}

// StartAgency Send bets to the NationalLotteryCenter
func (lottery *LotteryAgency) StartAgency() {
	lottery.client.StartClientLoop(lottery.bets)
}
