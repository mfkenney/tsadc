// Tsadc package provides an interface to the ADC on a
// Technologic Systems SOC using the tsctl server.
package tsadc

import (
	"apl.uw.edu/mikek/tsctl"
	"fmt"
	"net"
)

type Adc struct {
	conn       net.Conn
	baseaddr   uint32
	chans      uint16
	max_counts int32
}

// Base address is relative to the SYSCON bus
const ts4200_base uint32 = 0x80
const ts4800_base uint32 = 0x6000
const cfgreg uint32 = 0
const maskreg uint32 = 2
const datareg uint32 = 4

var adc_gain = map[uint]uint32{
	0: 0,
	2: 1,
	4: 2,
	8: 3,
}
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
	err = tsctl.UnpackReply(conn, &reply)

	return err
}

// NewAdc returns a new Adc with the specified bit-width and gain value
func NewAdc(base uint32, chans []uint, bits, gain uint) (*Adc, error) {
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

	// Set the channel mask
	for _, c := range chans {
		adc.chans |= (1 << (c - 1))
	}

	var reply tsctl.TsReply

	// Build the message to set the configuration register
	buf, _ := tsctl.PokeMsg(adc.baseaddr+cfgreg,
		16,
		uint32(0xad10|bits_val|gain_val))

	// Send the message
	err = send_msg(conn, buf, &reply)
	if err != nil {
		return nil, err
	}

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
	return NewAdc(ts4200_base, chans, bits, gain)
}

// Version of NewAdc for the TS-4800 CPU board
func NewTs4800Adc(chans []uint, bits, gain uint) (*Adc, error) {
	return NewAdc(ts4800_base, chans, bits, gain)
}

// ReadChan returns the A/D value from the specified channel
func (adc *Adc) ReadChan(c uint16) (int16, error) {
	if (adc.chans & (1 << (c - 1))) == 0 {
		return 0, fmt.Errorf("Invalid channel: %d", c)
	}

	var reply tsctl.ScalarReply
	buf, _ := tsctl.PeekMsg(adc.baseaddr+datareg, 16)
	err := send_msg(adc.conn, buf, &reply)
	if err != nil {
		return 0, err
	}
	return int16(reply.Value), nil
}

// ToVolts converts an A/D value from counts to volts
func (adc *Adc) ToVolts(val int16) float32 {
	return float32(val) * 10.24 / float32(adc.max_counts)
}