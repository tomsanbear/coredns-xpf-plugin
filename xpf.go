package xpf

import (
	"fmt"
	"net"
	"strconv"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnstest"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

var log = clog.NewWithPlugin("xpf")

// DefaultTypeXPF uses the default rrtype used in wireshark
const DefaultTypeXPF uint16 = 65422

// XPF type captures anything needed to append the XPF record to our queries
type XPF struct {
	rrtype uint16

	Next plugin.Handler
}

// New creates a new instance of the XPF type
func New() (*XPF, error) {
	return &XPF{rrtype: DefaultTypeXPF}, nil
}

func (xpf *XPF) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (rc int, err error) {
	state := request.Request{W: w, Req: r}
	rrw := dnstest.NewRecorder(w)

	err = appendXpfRecord(xpf.rrtype, &state)
	if err != nil {
		log.Errorf("xpf append failed with: %v", err)
		return rc, &Error{"failed to append the XPF record to the DNS request"}
	}
	rc, err = plugin.NextOrFailure(xpf.Name(), xpf.Next, ctx, rrw, r)
	return rc, err
}

func (xpf *XPF) Name() string { return "xpf" }

// AppendXpfRecord adds the relevant XPF record to the request object
func appendXpfRecord(rrtype uint16, state *request.Request) error {
	xpfRR := &dns.PrivateRR{}

	xpfRRData := &XPFPrivateRR{}
	switch state.Family() {
	case 1:
		xpfRRData.IPVersion = 4
		xpfRRData.SrcAddress = net.ParseIP(state.IP()).To4()
		xpfRRData.DestAddress = net.ParseIP(state.LocalIP()).To4()
	case 2:
		xpfRRData.IPVersion = 6
		xpfRRData.SrcAddress = net.ParseIP(state.IP()).To16()
		xpfRRData.DestAddress = net.ParseIP(state.LocalIP()).To16()
	}
	srcPort64, err := strconv.ParseUint(state.Port(), 10, 16)
	if err != nil {
		return err
	}
	xpfRRData.SrcPort = uint16(srcPort64)
	destPort64, err := strconv.ParseUint(state.LocalPort(), 10, 16)
	if err != nil {
		return err
	}
	xpfRRData.DestPort = uint16(destPort64)
	xpfRRData.Protocol, err = protoIANA(state.Proto())
	if err != nil {
		return err
	}

	// Put the data into the PrivateRR
	xpfRR.Data = xpfRRData

	xpfRR.Hdr = dns.RR_Header{
		Name:   ".",
		Rrtype: rrtype,
		Class:  1,
		Ttl:    0,
	}

	// Append to the Additional Section
	state.Req.Extra = append(state.Req.Extra, xpfRR)

	return nil
}

func protoIANA(proto string) (uint8, error) {
	switch proto {
	case "udp":
		return 17, nil
	case "tcp":
		return 6, nil
	}
	return 0, fmt.Errorf("invalid network protocol: %v", proto)
}

// OnStartup handles any plugin specific startup logic
func (x *XPF) OnStartup() (err error) {
	// Setup up the new record type
	dns.PrivateHandle("XPF", x.rrtype, NewXPFPrivateRR)
	return nil
}

// OnShutdown handles any plugin specific startup logic
func (x *XPF) OnShutdown() (err error) {
	dns.PrivateHandleRemove(x.rrtype)
	return nil
}
