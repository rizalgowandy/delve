package dwarf

import (
	"bytes"
	"debug/dwarf"
	"encoding/binary"
	"fmt"
	"io"
)

// ReadString reads a null-terminated string from data.
func ReadString(data *bytes.Buffer) (string, error) {
	str, err := data.ReadString(0x0)
	if err != nil {
		return "", err
	}

	return str[:len(str)-1], nil
}

// ReadUintRaw reads an integer of ptrSize bytes, with the specified byte order, from reader.
func ReadUintRaw(reader io.Reader, order binary.ByteOrder, ptrSize int) (uint64, error) {
	switch ptrSize {
	case 2:
		var n uint16
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return uint64(n), nil
	case 4:
		var n uint32
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return uint64(n), nil
	case 8:
		var n uint64
		if err := binary.Read(reader, order, &n); err != nil {
			return 0, err
		}
		return n, nil
	}
	return 0, fmt.Errorf("pointer size %d not supported", ptrSize)
}

// WriteUint writes an integer of ptrSize bytes to writer, in the specified byte order.
func WriteUint(writer io.Writer, order binary.ByteOrder, ptrSize int, data uint64) error {
	switch ptrSize {
	case 4:
		return binary.Write(writer, order, uint32(data))
	case 8:
		return binary.Write(writer, order, data)
	}
	return fmt.Errorf("pointer size %d not supported", ptrSize)
}

// ReadDwarfLengthVersion reads a DWARF length field followed by a version field
func ReadDwarfLengthVersion(data []byte) (length uint64, dwarf64 bool, version uint8, byteOrder binary.ByteOrder) {
	if len(data) < 4 {
		return 0, false, 0, binary.LittleEndian
	}

	lengthfield := binary.LittleEndian.Uint32(data)
	voff := 4
	if lengthfield == ^uint32(0) {
		dwarf64 = true
		voff = 12
	}

	if voff+1 >= len(data) {
		return 0, false, 0, binary.LittleEndian
	}

	byteOrder = binary.LittleEndian
	x, y := data[voff], data[voff+1]
	switch {
	default:
		fallthrough
	case x == 0 && y == 0:
		version = 0
		byteOrder = binary.LittleEndian
	case x == 0:
		version = y
		byteOrder = binary.BigEndian
	case y == 0:
		version = x
		byteOrder = binary.LittleEndian
	}

	if dwarf64 {
		length = byteOrder.Uint64(data[4:])
	} else {
		length = uint64(byteOrder.Uint32(data))
	}

	return length, dwarf64, version, byteOrder
}

const (
	_DW_UT_compile = 0x1 + iota
	_DW_UT_type
	_DW_UT_partial
	_DW_UT_skeleton
	_DW_UT_split_compile
	_DW_UT_split_type
)

// ReadUnitVersions reads the DWARF version of each unit in a debug_info section and returns them as a map.
func ReadUnitVersions(data []byte) map[dwarf.Offset]uint8 {
	r := make(map[dwarf.Offset]uint8)
	off := dwarf.Offset(0)
	for len(data) > 0 {
		length, dwarf64, version, _ := ReadDwarfLengthVersion(data)

		data = data[4:]
		off += 4
		secoffsz := 4
		if dwarf64 {
			off += 8
			secoffsz = 8
			data = data[8:]
		}

		var headerSize int

		switch version {
		case 2, 3, 4:
			headerSize = 3 + secoffsz
		default: // 5 and later?
			unitType := data[2]

			switch unitType {
			case _DW_UT_compile, _DW_UT_partial:
				headerSize = 4 + secoffsz

			case _DW_UT_skeleton, _DW_UT_split_compile:
				headerSize = 4 + secoffsz + 8

			case _DW_UT_type, _DW_UT_split_type:
				headerSize = 4 + secoffsz + 8 + secoffsz
			}
		}

		r[off+dwarf.Offset(headerSize)] = version

		data = data[length:] // skip contents
		off += dwarf.Offset(length)
	}
	return r
}
