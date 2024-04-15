package main

import (
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

	return RegisterToRegisterMovState, nil
}

func RegisterToRegisterMovState(params *DecoderParams) (DecoderState, error) {
	byte1 := params.Data[params.Pointer]
	byte2 := params.Data[params.Pointer+1]

	opcode := byte1 >> 2

	if opcode != 0b100010 {
		return nil, fmt.Errorf("Unexpected opcode: %b", opcode)
	}

	destinationBit := (byte1 >> 1) & 0b1
	wideBit := (byte1 >> 0) & 0b1
	mode := (byte2 >> 6) & 0b11
	_ = mode
	reg := (byte2 >> 3) & 0b111
	rm := (byte2 >> 0) & 0b111

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
