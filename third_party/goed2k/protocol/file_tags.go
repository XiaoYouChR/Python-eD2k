package protocol

const (
	FTFilename        byte = 0x01
	FTFileSize        byte = 0x02
	FTFileType        byte = 0x03
	FTFileFormat      byte = 0x04
	FTSources         byte = 0x15
	FTCompleteSources byte = 0x30
	FTFileSizeHi      byte = 0x3A
	FTMediaLength     byte = 0xD3
	FTMediaBitrate    byte = 0xD4
	FTMediaCodec      byte = 0xD5
)
