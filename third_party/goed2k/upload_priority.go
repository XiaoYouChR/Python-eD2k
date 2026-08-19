package goed2k

type UploadPriority int

const (
	UploadPriorityVeryLow UploadPriority = iota
	UploadPriorityLow
	UploadPriorityNormal
	UploadPriorityHigh
	UploadPriorityVeryHigh
	UploadPriorityPowerShare
)

func (p UploadPriority) ScoreFactor() float64 {
	switch p {
	case UploadPriorityPowerShare:
		return 250.0
	case UploadPriorityVeryHigh:
		return 1.8
	case UploadPriorityHigh:
		return 0.9
	case UploadPriorityLow:
		return 0.6
	case UploadPriorityVeryLow:
		return 0.2
	case UploadPriorityNormal:
		fallthrough
	default:
		return 0.7
	}
}
