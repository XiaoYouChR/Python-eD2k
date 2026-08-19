package protocol

import "bytes"

type WithoutDataPacket struct{}

func (w *WithoutDataPacket) Get(_ *bytes.Reader) error { return nil }
func (w WithoutDataPacket) Put(_ *bytes.Buffer) error  { return nil }
func (w WithoutDataPacket) BytesCount() int            { return 0 }
