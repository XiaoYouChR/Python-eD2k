package goed2k

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func Byte2String(value []byte) string {
	if value == nil {
		return ""
	}
	return strings.ToUpper(hex.EncodeToString(value))
}

func IP2String(ip int32) string {
	return fmt.Sprintf("%d.%d.%d.%d", byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24))
}

func String2IP(s string) (int32, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return 0, NewError(IllegalArgument)
	}

	raw := make([]int, 4)
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || value > 255 {
			return 0, NewError(IllegalArgument)
		}
		raw[i] = value
	}

	return int32(
		uint32(raw[0]) |
			(uint32(raw[1]) << 8) |
			(uint32(raw[2]) << 16) |
			(uint32(raw[3]) << 24),
	), nil
}

func Int2Address(ip int32) net.IP {
	return net.IPv4(byte(ip), byte(ip>>8), byte(ip>>16), byte(ip>>24)).To4()
}

func HTONLBytes(order []byte) int32 {
	if len(order) != 4 {
		return 0
	}
	return int32(order[0])<<24 | int32(order[1])<<16 | int32(order[2])<<8 | int32(order[3])
}

func HTONL(ip int32) int32 {
	return ((ip & 0x000000FF) << 24) |
		((ip & 0x0000FF00) << 8) |
		((ip >> 8) & 0x0000FF00) |
		((ip >> 24) & 0x000000FF)
}

func PackToNetworkByteOrder(order []byte) int32 {
	if len(order) != 4 {
		return 0
	}
	return int32(order[3])<<24 | int32(order[2])<<16 | int32(order[1])<<8 | int32(order[0])
}

func NTOHL(ip int32) int32 {
	raw := []byte{byte(ip), byte(ip >> 8), byte(ip >> 16), byte(ip >> 24)}
	return HTONLBytes(raw)
}

func IsLocalAddress(ip int32) bool {
	host := uint32(NTOHL(ip))
	return (host&0xff000000) == 0x0a000000 ||
		(host&0xfff00000) == 0xac100000 ||
		(host&0xffff0000) == 0xc0a80000 ||
		(host&0xffff0000) == 0xa9fe0000 ||
		(host&0xff000000) == 0x7f000000
}

func LowPart(value int64) int32 {
	return int32(value)
}

func HiPart(value int64) int32 {
	return int32(value >> 32)
}

func MakeFullED2KVersion(clientID, a, b, c int64) int64 {
	return (clientID << 24) | (a << 17) | (b << 10) | (c << 7)
}

func DivCeil(a, b int64) int64 {
	return (a + b - 1) / b
}

func IsLowID(v int32) bool {
	return int64(uint32(v)) < HighestLowIDED2K
}

func FormatLink(fileName string, fileSize int64, hash fmt.Stringer) string {
	return "ed2k://|file|" + strings.ReplaceAll(fileName, " ", "%20") + "|" + strconv.FormatInt(fileSize, 10) + "|" + hash.String() + "|/"
}

func IsBit(value, mask int32) bool {
	return (value & mask) == mask
}

func formatMessage(format string, args ...any) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
