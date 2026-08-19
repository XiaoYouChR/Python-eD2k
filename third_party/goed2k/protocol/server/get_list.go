package server

import "bytes"

type GetList struct{}

func (g *GetList) Get(_ *bytes.Reader) error { return nil }
func (g GetList) Put(_ *bytes.Buffer) error  { return nil }
func (g GetList) BytesCount() int            { return 0 }
