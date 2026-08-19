package goed2k

const (
	InvalidSpeed int64 = -1
)

type SpeedMonitor struct {
	speedSamples []int64
	roundRobin   int
	totalSamples int
}

func NewSpeedMonitor(samplesLimit int) SpeedMonitor {
	return SpeedMonitor{speedSamples: make([]int64, samplesLimit)}
}

func (s *SpeedMonitor) AddSample(speedSample int64) {
	if s.roundRobin == len(s.speedSamples) {
		s.roundRobin = 0
	}
	if s.roundRobin < len(s.speedSamples) {
		s.speedSamples[s.roundRobin] = speedSample
		s.roundRobin++
	}
	if s.totalSamples != len(s.speedSamples) {
		s.totalSamples++
	}
}

func (s SpeedMonitor) AverageSpeed() int64 {
	if s.totalSamples == 0 {
		return InvalidSpeed
	}
	var sum int64
	for i := 0; i < s.totalSamples; i++ {
		sum += s.speedSamples[i]
	}
	return sum / int64(s.totalSamples)
}

func (s SpeedMonitor) NumSamples() int {
	return s.totalSamples
}

func (s *SpeedMonitor) Clear() {
	s.roundRobin = 0
	s.totalSamples = 0
}
