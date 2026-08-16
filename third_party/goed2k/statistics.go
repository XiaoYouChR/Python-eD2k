package goed2k

const (
	uploadPayload = iota
	uploadProtocol
	downloadPayload
	downloadProtocol
	channelsCount
)

type Statistics struct {
	channels [channelsCount]StatChannel
}

func NewStatistics() Statistics {
	var s Statistics
	for i := range s.channels {
		s.channels[i] = NewStatChannel()
	}
	return s
}

func (s *Statistics) Add(other Statistics) *Statistics {
	for i := range s.channels {
		s.channels[i].AddChannel(other.channels[i])
	}
	return s
}

func (s *Statistics) Merge(other Statistics) *Statistics {
	for i := range s.channels {
		s.channels[i].MergeChannel(other.channels[i])
	}
	return s
}

func (s *Statistics) SecondTick(timeIntervalMS int64) {
	for i := range s.channels {
		s.channels[i].SecondTick(timeIntervalMS)
	}
}

func (s *Statistics) Clear() {
	for i := range s.channels {
		s.channels[i].Clear()
	}
}

func (s *Statistics) ReceiveBytes(protocolBytes, payloadBytes int64) {
	s.channels[downloadProtocol].Add(protocolBytes)
	s.channels[downloadPayload].Add(payloadBytes)
}

func (s *Statistics) SendBytes(protocolBytes, payloadBytes int64) {
	s.channels[uploadProtocol].Add(protocolBytes)
	s.channels[uploadPayload].Add(payloadBytes)
}

func (s Statistics) TotalPayloadDownload() int64 {
	return s.channels[downloadPayload].Total()
}

func (s Statistics) TotalProtocolDownload() int64 {
	return s.channels[downloadProtocol].Total()
}

func (s Statistics) TotalPayloadUpload() int64 {
	return s.channels[uploadPayload].Total()
}

func (s Statistics) TotalProtocolUpload() int64 {
	return s.channels[uploadProtocol].Total()
}

func (s Statistics) TotalUpload() int64 {
	return s.channels[uploadPayload].Total() + s.channels[uploadProtocol].Total()
}

func (s Statistics) LastDownload() int64 {
	return s.channels[downloadPayload].Counter() + s.channels[downloadProtocol].Counter()
}

func (s Statistics) LastUpload() int64 {
	return s.channels[uploadPayload].Counter() + s.channels[uploadProtocol].Counter()
}

func (s Statistics) DownloadRate() int64 {
	return s.channels[downloadPayload].Rate() + s.channels[downloadProtocol].Rate()
}

func (s Statistics) DownloadPayloadRate() int64 {
	return s.channels[downloadPayload].Rate()
}

func (s Statistics) UploadRate() int64 {
	return s.channels[uploadPayload].Rate() + s.channels[uploadProtocol].Rate()
}

func (s Statistics) UploadPayloadRate() int64 {
	return s.channels[uploadPayload].Rate()
}

func (s Statistics) LowPassUploadRate() int64 {
	return s.channels[uploadPayload].LowPassRate() + s.channels[uploadProtocol].LowPassRate()
}

func (s Statistics) LowPassDownloadRate() int64 {
	return s.channels[downloadPayload].LowPassRate() + s.channels[downloadProtocol].LowPassRate()
}
