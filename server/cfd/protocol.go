package cfd

import (
	"io"
)

type protocolSignature [6]byte

var (
	dataStreamProtocolSignature = protocolSignature{0x0A, 0x36, 0xCD, 0x12, 0xA1, 0x3E}
)

type protocolVersion string

const (
	protocolV1 protocolVersion = "01"

	protocolVersionLength = 2
)

func readVersion(stream io.Reader) (string, error) {
	version := make([]byte, 2)
	_, err := stream.Read(version)
	return string(version), err
}

func writeVersion(stream io.Writer) error {
	_, err := stream.Write([]byte(protocolV1)[:protocolVersionLength])
	return err
}

func writeDataStreamPreamble(stream io.Writer) error {
	if err := writeSignature(stream, dataStreamProtocolSignature); err != nil {
		return err
	}

	return writeVersion(stream)
}

func writeSignature(stream io.Writer, signature protocolSignature) error {
	_, err := stream.Write(signature[:])
	return err
}
