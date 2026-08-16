package goed2k

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/monkeyWie/goed2k/protocol"
)

type LinkType string

const (
	LinkServer  LinkType = "SERVER"
	LinkServers LinkType = "SERVERS"
	LinkNodes   LinkType = "NODES"
	LinkFile    LinkType = "FILE"
)

type EMuleLink struct {
	Hash        protocol.Hash
	NumberValue int64
	StringValue string
	Type        LinkType
}

func ParseEMuleLink(uri string) (EMuleLink, error) {
	if uri == "" {
		return EMuleLink{}, NewError(LinkMailformed)
	}

	decURI, err := url.QueryUnescape(uri)
	if err != nil {
		return EMuleLink{}, NewError(UnsupportedEncoding)
	}

	parts := strings.Split(decURI, "|")
	if len(parts) < 2 || parts[0] != "ed2k://" || parts[len(parts)-1] != "/" {
		return EMuleLink{}, NewError(LinkMailformed)
	}

	switch parts[1] {
	case "server":
		if len(parts) != 5 {
			return EMuleLink{}, NewError(LinkMailformed)
		}
		port, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return EMuleLink{}, NewError(NumberFormatError)
		}
		return EMuleLink{NumberValue: port, StringValue: parts[2], Type: LinkServer}, nil
	case "serverlist":
		if len(parts) != 4 {
			return EMuleLink{}, NewError(LinkMailformed)
		}
		return EMuleLink{StringValue: parts[2], Type: LinkServers}, nil
	case "nodeslist":
		if len(parts) != 4 {
			return EMuleLink{}, NewError(LinkMailformed)
		}
		return EMuleLink{StringValue: parts[2], Type: LinkNodes}, nil
	case "file":
		if len(parts) < 6 {
			return EMuleLink{}, NewError(LinkMailformed)
		}
		hash, err := protocol.HashFromString(parts[4])
		if err != nil {
			return EMuleLink{}, NewError(InternalError)
		}
		size, err := strconv.ParseInt(parts[3], 10, 64)
		if err != nil {
			return EMuleLink{}, NewError(NumberFormatError)
		}
		name, err := url.QueryUnescape(parts[2])
		if err != nil {
			return EMuleLink{}, NewError(UnsupportedEncoding)
		}
		return EMuleLink{
			Hash:        hash,
			NumberValue: size,
			StringValue: name,
			Type:        LinkFile,
		}, nil
	}

	return EMuleLink{}, NewError(UnknownLinkType)
}
