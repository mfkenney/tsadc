// Tsadc package provides an interface to the ADC on a
// Technologic Systems SOC using the tsctl server.
package tsadc

import (
	"apl.uw.edu/mikek/tsctl"
	"fmt"
	"net"
)

// Base address is relative to the FPGA
const ts4200_base uint32 = 0x80
const ts4800_base uint32 = 0x6000
const cfgreg uint32 = 0
const maskreg uint32 = 2
const datareg uint32 = 4
const max_chans = 6

type Adc struct {
	conn       net.Conn
	baseaddr   uint32
	chans      uint
	max_counts int32
	max_volts  [max_chans]float32
}

// Map gain multipliers to register setting
var adc_gain = map[uint]uint32{
	0: 0,
	2: 1,
	4: 2,
	8: 3,
}

// Map sample size to register setting
var adc_bits = map[uint]uint32{
	12: 0,
	14: (1 << 2),
	16: (2 << 2),
}

func send_msg(conn net.Conn, buf []byte, reply interface{}) error {
	// Send the message
	_, err := conn.Write(buf)
	if err != nil {
		return err
	}

	// Read the reply
	err = tsctl.UnpackReply(conn, reply)

	return err
}

// NewAdc returns a new Adc with the specified bit-width and gain value
func NewAdc(base uint32, an_sel uint32,
	chans []uint, bits, gain uint) (*Adc, error) {

	bits_val, ok := adc_bits[bits]
	if !ok {
		return nil, fmt.Errorf("Invalid bits setting: %d", bits)
	}
	gain_val, ok := adc_gain[gain]
	if !ok {
		return nil, fmt.Errorf("Invalid gain setting: %d", gain)
	}

	conn, err := net.Dial("tcp", "localhost:5001")
	if err != nil {
		return nil, err
	}

	adc := Adc{baseaddr: base, conn: conn}
	adc.max_counts = 1 << (bits - 1)
	adc.max_volts = [max_chans]float32{2.048, 2.048, 10.24, 10.24, 10.24, 10.24}

	// Set the channel mask
	for _, c := range chans {
		adc.chans |= (1 << (c - 1))
	}

	var reply tsctl.TsReply

	// Build the message to set the configuration register
	buf, _ := tsctl.PokeMsg(adc.baseaddr+cfgreg,
		16,
		uint32(an_sel|bits_val|gain_val))

	// Send the message
	err = send_msg(conn, buf, &reply)
	if err != nil {
		return nil, err
	}

	// Build a message toset the channel-mask register
	buf, _ = tsctl.PokeMsg(adc.baseaddr+maskreg,
		16,
		uint32(adc.chans))

	// Send the message
	err = send_msg(conn, buf, &reply)
	if err != nil {
		return nil, err
	}

	return &adc, nil
}

// Version of NewAdc for the TS-4200 CPU board
func NewTs4200Adc(chans []uint, bits, gain uint) (*Adc, error) {
	return NewAdc(ts4200_base, 0x10, chans, bits, gain)
}

// Version of NewAdc for the TS-4800 CPU board
func NewTs4800Adc(chans []uint, bits, gain uint) (*Adc, error) {
	return NewAdc(ts4800_base, 0x0, chans, bits, gain)
}

// ReadChan returns the A/D value from the specified channel
func (adc *Adc) ReadCounts(c uint) (int16, error) {
	if (adc.chans & (1 << (c - 1))) == 0 {
		return 0, fmt.Errorf("Invalid channel: %d", c)
	}

	var offset uint32 = 2 * (uint32(c) - 1)
	var reply tsctl.ScalarReply
	buf, _ := tsctl.PeekMsg(adc.baseaddr+datareg+offset, 16)
	err := send_msg(adc.conn, buf, &reply)
	if err != nil {
		return 0, err
	}
	return int16(reply.Value), nil
}

// ReadVolts returns the A/D value in volts
func (adc *Adc) ReadVolts(c uint) (float32, error) {
	val, err := adc.ReadCounts(c)
	if err != nil {
		return 0., err
	}
	return float32(val) * adc.max_volts[c-1] / float32(adc.max_counts), nil
}
