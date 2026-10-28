package display

import "tinygo.org/x/drivers"

// BlockingSPI uses the RP2040's synchronous, full-duplex SPI path, including for
// writes. This avoids the DMA send-only path at command/data and chip-select edges.
// All discarded input stays in one small scratch buffer; no receive wire is needed.
type BlockingSPI struct {
	bus     drivers.SPI
	discard [64]byte
}

func NewBlockingSPI(bus drivers.SPI) *BlockingSPI { return &BlockingSPI{bus: bus} }

func (s *BlockingSPI) Tx(w, r []byte) error {
	if r != nil {
		return s.bus.Tx(w, r)
	}
	for len(w) > 0 {
		n := min(len(w), len(s.discard))
		if err := s.bus.Tx(w[:n], s.discard[:n]); err != nil {
			return err
		}
		w = w[n:]
	}
	return nil
}
func (s *BlockingSPI) Transfer(b byte) (byte, error) { return s.bus.Transfer(b) }
