package goed2k

import "fmt"

type BaseErrorCode interface {
	Code() int
	Description() string
}

type ErrorCode int

const (
	NoError ErrorCode = iota
	ServerConnUnsupportedPacket
	PeerConnUnsupportedPacket
	PacketHeaderUndefined
	InflateError
	PacketSizeIncorrect
	PacketSizeOverflow
	ServerMetHeaderIncorrect
	GenericInstantiationError
	GenericIllegalAccess

	EndOfStream              ErrorCode = 10
	IOException              ErrorCode = 11
	NoTransfer               ErrorCode = 12
	FileNotFound             ErrorCode = 13
	OutOfParts               ErrorCode = 14
	ConnectionTimeout        ErrorCode = 15
	ChannelClosed            ErrorCode = 16
	QueueRanking             ErrorCode = 17
	FileIOError              ErrorCode = 18
	UnableToDeleteFile       ErrorCode = 19
	InternalError            ErrorCode = 20
	BufferUnderflowException ErrorCode = 21
	BufferGetException       ErrorCode = 22
	WrongHashSet             ErrorCode = 23
	HashMismatch             ErrorCode = 24
	NonWriteableChannel      ErrorCode = 25

	TagTypeUnknown         ErrorCode = 30
	TagToStringInvalid     ErrorCode = 31
	TagToIntInvalid        ErrorCode = 32
	TagToLongInvalid       ErrorCode = 33
	TagToFloatInvalid      ErrorCode = 34
	TagToHashInvalid       ErrorCode = 35
	TagFromStringInvalidCP ErrorCode = 36
	TagToBlobInvalid       ErrorCode = 37
	TagToBSOBInvalid       ErrorCode = 38

	DuplicatePeer           ErrorCode = 40
	DuplicatePeerConnection ErrorCode = 41
	PeerLimitExceeded       ErrorCode = 42
	SecurityException       ErrorCode = 43
	UnsupportedEncoding     ErrorCode = 44
	IllegalArgument         ErrorCode = 45

	TransferFinished ErrorCode = 50
	TransferPaused   ErrorCode = 51
	TransferAborted  ErrorCode = 52

	NoMemory                ErrorCode = 60
	SessionStopping         ErrorCode = 61
	IncomingDirInaccessible ErrorCode = 62
	BufferTooLarge          ErrorCode = 63
	NotConnected            ErrorCode = 64
	Interrupted             ErrorCode = 65

	PortMappingAlreadyMapped   ErrorCode = 70
	PortMappingNoDevice        ErrorCode = 71
	PortMappingError           ErrorCode = 72
	PortMappingIOError         ErrorCode = 73
	PortMappingSAXError        ErrorCode = 74
	PortMappingConfigError     ErrorCode = 75
	PortMappingException       ErrorCode = 76
	PortMappingCommandRejected ErrorCode = 77

	DHTRequestAlreadyRunning ErrorCode = 80
	DHTTrackerAborted        ErrorCode = 81

	LinkMailformed         ErrorCode = 90
	URISyntaxError         ErrorCode = 91
	NumberFormatError      ErrorCode = 92
	UnknownLinkType        ErrorCode = 93
	GithubCfgIPIsNull      ErrorCode = 94
	GithubCfgPortsAreNull  ErrorCode = 95
	GithubCfgPortsAreEmpty ErrorCode = 96
	InvalidPRParameter     ErrorCode = 97
	PeerRequestOverflow    ErrorCode = 98

	Fail ErrorCode = 100
)

var errorDescriptions = map[ErrorCode]string{
	NoError:                     "No error",
	ServerConnUnsupportedPacket: "Server unsupported packet",
	PeerConnUnsupportedPacket:   "Peer connection unsupported packet",
	PacketHeaderUndefined:       "Packet header contains wrong bytes or undefined",
	InflateError:                "Inflate error",
	PacketSizeIncorrect:         "Packet size less than zero",
	PacketSizeOverflow:          "Packet size too big",
	ServerMetHeaderIncorrect:    "Server met file contains incorrect header byte",
	GenericInstantiationError:   "Generic instantiation error",
	GenericIllegalAccess:        "Generic illegal access",
	EndOfStream:                 "End of stream",
	IOException:                 "I/O exception",
	NoTransfer:                  "No transfer",
	FileNotFound:                "File not found",
	OutOfParts:                  "Out of parts",
	ConnectionTimeout:           "Connection timeout",
	ChannelClosed:               "Channel closed",
	QueueRanking:                "Queue ranking",
	FileIOError:                 "File I/O error occured",
	UnableToDeleteFile:          "Unable to delete file",
	InternalError:               "Internal product error",
	BufferUnderflowException:    "Buffer underflow exception",
	BufferGetException:          "Buffer get method raised common exception",
	WrongHashSet:                "Wrong getHash set",
	HashMismatch:                "Hash mismatch",
	NonWriteableChannel:         "Non writeable channel ",
	TagTypeUnknown:              "Tag type unknown",
	TagToStringInvalid:          "Tag to string convertion error",
	TagToIntInvalid:             "Tag to int conversion error",
	TagToLongInvalid:            "Tag to long conversion error",
	TagToFloatInvalid:           "Tag to float conversion error",
	TagToHashInvalid:            "Tag to getHash conversion error",
	TagFromStringInvalidCP:      "Tag from string creation error invalid code page",
	TagToBlobInvalid:            "Tag to blob conversion error",
	TagToBSOBInvalid:            "Tag to bsob coversion error",
	DuplicatePeer:               "Duplicate peer",
	DuplicatePeerConnection:     "Duplicate peer connection",
	PeerLimitExceeded:           "Peer limit exeeded",
	SecurityException:           "Security exception",
	UnsupportedEncoding:         "Unsupported encoding exception",
	IllegalArgument:             "Illegal argument",
	TransferFinished:            "Transfer finished",
	TransferPaused:              "Transfer paused",
	TransferAborted:             "Transfer aborted",
	NoMemory:                    "No memory available",
	SessionStopping:             "Session stopping",
	IncomingDirInaccessible:     "Incoming directory is inaccessible",
	BufferTooLarge:              "Buffer too large",
	NotConnected:                "Not connected",
	Interrupted:                 "Interrupted",
	PortMappingAlreadyMapped:    "Port already mapped",
	PortMappingNoDevice:         "No gateway device found",
	PortMappingError:            "Unable to map port",
	PortMappingIOError:          "I/O exception on mapping port",
	PortMappingSAXError:         "SAX parsing exception on port mapping",
	PortMappingConfigError:      "Configuration exception on port mapping",
	PortMappingException:        "Unknown exception on port mapping",
	PortMappingCommandRejected:  "Mapping command was rejected",
	DHTRequestAlreadyRunning:    "DHT request with the same getHash already in progress",
	DHTTrackerAborted:           "DHT tracker was already aborted at the moment",
	LinkMailformed:              "Incorrect link format",
	URISyntaxError:              "URI has incorrect syntax",
	NumberFormatError:           "Parse number exception",
	UnknownLinkType:             "Emule link has unrecognized type",
	GithubCfgIPIsNull:           "Ip is null in github kad config",
	GithubCfgPortsAreNull:       "Ports are null in github kad config",
	GithubCfgPortsAreEmpty:      "Ports are empty in github kad config",
	InvalidPRParameter:          "Peer request parameters are invalid",
	PeerRequestOverflow:         "Peer request has length greater than PIECE_SIZE",
	Fail:                        "Fail",
}

func (e ErrorCode) Code() int {
	return int(e)
}

func (e ErrorCode) Description() string {
	if desc, ok := errorDescriptions[e]; ok {
		return desc
	}
	return "Unknown error"
}

func (e ErrorCode) String() string {
	return fmt.Sprintf("%s {%d}", e.Description(), e.Code())
}
