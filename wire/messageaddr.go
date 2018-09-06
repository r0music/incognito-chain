package wire

import (
	"encoding/json"
)

type MessageAddr struct {
	RawAddresses []string
}

func (self MessageAddr) MessageType() string {
	return CmdAddr
}

func (self MessageAddr) MaxPayloadLength(pver int) int {
	return MaxBlockPayload
}

func (self MessageAddr) JsonSerialize() ([]byte, error) {
	jsonBytes, err := json.Marshal(self)
	return jsonBytes, err
}

func (self MessageAddr) JsonDeserialize(jsonStr string) error {
	err := json.Unmarshal([]byte(jsonStr), self)
	return err
}
