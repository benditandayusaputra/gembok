package chip

import (
	"fmt"
	"time"
)

const (
	MinWinLog2 = 1
	MaxWinLog2 = 20

	pufVoteOverheadCycles = 32
	pufPairOverheadCycles = 16
	pufBudgetSlackCycles  = 100_000
	pufTimeoutMargin      = time.Second
)

type Caps struct {
	PUFMode     int
	Debug       bool
	WinLog2     int
	Votes       int
	Oscillators int
}

func DecodeCaps(raw uint32) Caps {
	return Caps{
		PUFMode:     int(raw & 3),
		Debug:       raw&4 != 0,
		WinLog2:     int(raw >> 3 & 0x1F),
		Votes:       int(raw >> 8 & 0xFF),
		Oscillators: int(raw >> 16 & 0xFFF),
	}
}

func (c Caps) Encode() uint32 {
	raw := uint32(c.Oscillators&0xFFF)<<16 | uint32(c.Votes&0xFF)<<8 | uint32(c.WinLog2&0x1F)<<3 | uint32(c.PUFMode&3)
	if c.Debug {
		raw |= 4
	}
	return raw
}

func (c Caps) Validate() error {
	if _, ok := HelperMaskLen(c.Oscillators); !ok {
		return fmt.Errorf("%w: %d osilator, harus %d sampai %d", ErrBadCaps, c.Oscillators, MinOscillators, MaxOscillators)
	}
	if c.WinLog2 < MinWinLog2 || c.WinLog2 > MaxWinLog2 {
		return fmt.Errorf("%w: WIN_LOG2 %d, harus %d sampai %d", ErrBadCaps, c.WinLog2, MinWinLog2, MaxWinLog2)
	}
	return nil
}

func (c Caps) MaskLen() int {
	n, _ := HelperMaskLen(c.Oscillators)
	return n
}

func (c Caps) PUFTimeout(floor time.Duration) time.Duration {
	return PUFTimeout(c.Oscillators, c.Votes, c.WinLog2, floor)
}

func PUFBudgetCycles(oscillators, votes, winLog2 int) int64 {
	window := int64(1) << winLog2
	perPair := int64(votes)*(window+pufVoteOverheadCycles) + pufPairOverheadCycles
	return int64(oscillators-1)*perPair + pufBudgetSlackCycles
}

func PUFTimeout(oscillators, votes, winLog2 int, floor time.Duration) time.Duration {
	limit := time.Duration(2*PUFBudgetCycles(oscillators, votes, winLog2))*CyclePeriod + pufTimeoutMargin
	return max(limit, floor)
}
