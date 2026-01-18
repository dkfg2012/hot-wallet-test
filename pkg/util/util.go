package util

import "fmt"

func ParseHexUint64(hexStr string) (uint64, error) {
	if len(hexStr) < 3 || hexStr[:2] != "0x" {
		return 0, fmt.Errorf("invalid hex quantity: %q", hexStr)
	}
	if hexStr == "0x0" {
		return 0, nil
	}
	var out uint64
	_, err := fmt.Sscanf(hexStr, "0x%x", &out)
	return out, err
}
