package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

/*
 Decoding the basic register-to-register 8086 MOV instruction

 Assembly:
  MOV ax, bx // Copy the value from register b to register a

 Machine code:
 byte 1            byte 2
 |x|x|x|x|x|x|x|x| |x|x|x|x|x|x|x|x|
 |Opcode     |D|W| |MOD|REG  |R/M  |

 D = Destination bit (if 1 REG is the destination, R/M otherwise)
 W = Wide (16 bit if 1, 8 bit otherwise)
 MOD = Mode
 REG = Register id
 R/M = Another register id if in register mode, a memory address otherwise

 MOV opcode = 100010
 Register-to-register MOD value = 11

 Reg table
 =========
 | REG | W=0 | W=1 |
 |-----------------|
 | 000 | AL  | AX  |
 | 001 | CL  | CX  |
 | 010 | DL  | DX  |
 | 011 | BL  | BX  |
 | 100 | AH  | SP  |
 | 101 | CH  | BP  |
 | 110 | DH  | SI  |
 | 111 | BH  | DI  |
*/

var RegisterNames map[byte]map[byte]string = map[byte]map[byte]string{
	0b000: map[byte]string{
		0: "al",
		1: "ax",
	},
	0b001: map[byte]string{
		0: "cl",
		1: "cx",
	},
	0b010: map[byte]string{
		0: "dl",
		1: "dx",
	},
	0b011: map[byte]string{
		0: "bl",
		1: "bx",
	},
	0b100: map[byte]string{
		0: "ah",
		1: "sp",
	},
	0b101: map[byte]string{
		0: "ch",
		1: "bp",
	},
	0b110: map[byte]string{
		0: "dh",
		1: "si",
	},
	0b111: map[byte]string{
		0: "bh",
		1: "di",
	},
}

var MemoryEquations map[byte]string = map[byte]string{
	0b000: "bx + si",
	0b001: "bx + di",
	0b010: "bp + si",
	0b011: "bp + di",
	0b100: "si",
	0b101: "di",
	0b110: "bp",
	0b111: "bx",
}

type DecoderParams struct {
	Data                []byte
	Pointer             int
	DecodedInstructions []string
}

type DecoderState func(params *DecoderParams) (DecoderState, error)

func InitialState(params *DecoderParams) (DecoderState, error) {
	bytesLeft := len(params.Data) - params.Pointer
	if bytesLeft == 0 {
		return nil, nil
	}
	if bytesLeft == 1 {
		return nil, fmt.Errorf("Trailing byte found")
	}

	nextByte := params.Data[params.Pointer]

	var nextState DecoderState
	if ((nextByte >> 2) ^ 0b100010) == 0 {
		nextState = RegAndRegOrMemState
	} else if ((nextByte >> 4) ^ 0b1011) == 0 {
		nextState = ImmediateToRegisterMovState
	} else {
		return nil, fmt.Errorf("Unknown opcode: %b", nextByte)
	}

	return nextState, nil
}

func RegAndRegOrMemState(params *DecoderParams) (DecoderState, error) {
	byte1 := params.Data[params.Pointer]
	byte2 := params.Data[params.Pointer+1]

	destinationBit := (byte1 >> 1) & 0b1
	wideBit := (byte1 >> 0) & 0b1
	mode := (byte2 >> 6) & 0b11

	reg := (byte2 >> 3) & 0b111
	rm := (byte2 >> 0) & 0b111

	switch mode {
	case 0b00:
		MemoryMode(
			destinationBit,
			wideBit,
			reg,
			rm,
			params,
		)
	case 0b01:
		MemoryMode8BitDisplace(
			destinationBit,
			wideBit,
			reg,
			rm,
			params,
		)
	case 0b10:
		MemoryMode16BitDisplace(
			destinationBit,
			wideBit,
			reg,
			rm,
			params,
		)
	case 0b11:
		RegisterMode(
			destinationBit,
			wideBit,
			reg,
			rm,
			params,
		)
	default:
		return nil, nil
	}

	return InitialState, nil
}

func MemoryMode(
	destinationBit byte,
	wideBit byte,
	reg byte,
	rm byte,
	params *DecoderParams,
) {
	location1 := fmt.Sprintf("[%s]", MemoryEquations[rm])
	location2 := RegisterNames[reg][wideBit]

	var destName, srcName string
	if destinationBit == 1 {
		destName = location2
		srcName = location1
	} else {
		destName = location1
		srcName = location2
	}

	params.DecodedInstructions = append(
		params.DecodedInstructions,
		fmt.Sprintf("mov %s, %s", destName, srcName),
	)
	params.Pointer += 2
}

func MemoryMode8BitDisplace(
	destinationBit byte,
	wideBit byte,
	reg byte,
	rm byte,
	params *DecoderParams,
) {
	data := binary.BigEndian.Uint16([]byte{
		0b0,
		params.Data[params.Pointer+2],
	})

	location1 := fmt.Sprintf(
		"[%s + %d]",
		MemoryEquations[rm],
		data,
	)
	location2 := RegisterNames[reg][wideBit]

	var destName, srcName string
	if destinationBit == 1 {
		destName = location2
		srcName = location1
	} else {
		destName = location1
		srcName = location2
	}

	params.DecodedInstructions = append(
		params.DecodedInstructions,
		fmt.Sprintf("mov %s, %s", destName, srcName),
	)
	params.Pointer += 3
}

func MemoryMode16BitDisplace(
	destinationBit byte,
	wideBit byte,
	reg byte,
	rm byte,
	params *DecoderParams,
) {
	data := binary.BigEndian.Uint16([]byte{
		params.Data[params.Pointer+3],
		params.Data[params.Pointer+2],
	})

	location1 := fmt.Sprintf(
		"[%s + %d]",
		MemoryEquations[rm],
		data,
	)
	location2 := RegisterNames[reg][wideBit]

	var destName, srcName string
	if destinationBit == 1 {
		destName = location2
		srcName = location1
	} else {
		destName = location1
		srcName = location2
	}

	params.DecodedInstructions = append(
		params.DecodedInstructions,
		fmt.Sprintf("mov %s, %s", destName, srcName),
	)
	params.Pointer += 4
}

func RegisterMode(
	destinationBit byte,
	wideBit byte,
	reg byte,
	rm byte,
	params *DecoderParams,
) {
	var destRegName, srcRegName string
	if destinationBit == 1 {
		destRegName = RegisterNames[reg][wideBit]
		srcRegName = RegisterNames[rm][wideBit]
	} else {
		destRegName = RegisterNames[rm][wideBit]
		srcRegName = RegisterNames[reg][wideBit]
	}

	params.DecodedInstructions = append(
		params.DecodedInstructions,
		fmt.Sprintf("mov %s, %s", destRegName, srcRegName),
	)
	params.Pointer += 2
}

func ImmediateToRegisterMovState(
	params *DecoderParams,
) (DecoderState, error) {
	byte1 := params.Data[params.Pointer]

	wideBit := (byte1 >> 3) & 0b1
	reg := (byte1 >> 0) & 0b111

	if wideBit == 1 {
		dataBytes := []byte{
			params.Data[params.Pointer+2],
			params.Data[params.Pointer+1],
		}
		data := binary.BigEndian.Uint16(dataBytes)

		destRegName := RegisterNames[reg][wideBit]

		params.DecodedInstructions = append(
			params.DecodedInstructions,
			fmt.Sprintf("mov %s, %d", destRegName, data),
		)
		params.Pointer += 3
	} else {
		dataBytes := []byte{
			0b0,
			params.Data[params.Pointer+1],
		}
		data := binary.BigEndian.Uint16(dataBytes)

		destRegName := RegisterNames[reg][wideBit]

		params.DecodedInstructions = append(
			params.DecodedInstructions,
			fmt.Sprintf("mov %s, %d", destRegName, data),
		)
		params.Pointer += 2
	}

	return InitialState, nil
}

func Decode(input []byte) []string {
	var state DecoderState = InitialState
	var params DecoderParams = DecoderParams{
		Data:                input,
		Pointer:             0,
		DecodedInstructions: []string{},
	}

	var err error
	for state != nil {
		state, err = state(&params)
		if err != nil {
			panic(err)
		}
	}

	return params.DecodedInstructions
}

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage: decoder <FILENAME>")
		os.Exit(1)
	}
	inputFilename := os.Args[1]
	inputFile, err := os.Open(inputFilename)
	if err != nil {
		panic(err)
	}

	inputData, err := io.ReadAll(inputFile)
	if err != nil {
		panic(err)
	}

	fmt.Println("bits 16\n")
	decodedInstructions := Decode(inputData)
	for _, instruction := range decodedInstructions {
		fmt.Println(instruction)
	}
}
