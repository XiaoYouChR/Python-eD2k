package goed2k

type StatChannel struct {
	secondCounter int64
	totalCounter  int64
	average5Sec   int64
	average30Sec  int64
	samples       []int64
}

func NewStatChannel() StatChannel {
	return StatChannel{samples: []int64{0, 0, 0, 0, 0}}
}

func (s *StatChannel) Add(count int64) {
	s.secondCounter += count
	s.totalCounter += count
}

func (s *StatChannel) Counter() int64 {
	return s.secondCounter
}

func (s *StatChannel) Total() int64 {
	return s.totalCounter
}

func (s *StatChannel) Rate() int64 {
	return s.average5Sec
}

func (s *StatChannel) LowPassRate() int64 {
	return s.average30Sec
}

func (s *StatChannel) Clear() {
	s.secondCounter = 0
	s.totalCounter = 0
	s.average5Sec = 0
	s.average30Sec = 0
	s.samples = []int64{0, 0, 0, 0, 0}
}

func (s *StatChannel) AddChannel(other StatChannel) {
	s.secondCounter += other.secondCounter
	s.totalCounter += other.totalCounter
}

func (s *StatChannel) MergeChannel(other StatChannel) {
	s.secondCounter += other.secondCounter
	s.totalCounter += other.totalCounter
	s.average5Sec += other.average5Sec
	s.average30Sec += other.average30Sec
}

func (s *StatChannel) SecondTick(timeIntervalMS int64) {
	sample := (s.secondCounter * 1000) / timeIntervalMS
	s.samples = append([]int64{sample}, s.samples...)
	if len(s.samples) > 5 {
		s.samples = s.samples[:5]
	}
	var sum int64
	for _, v := range s.samples {
		sum += v
	}
	s.average5Sec = sum / 5
	s.average30Sec = s.average30Sec*29/30 + sample/30
	s.secondCounter = 0
}
