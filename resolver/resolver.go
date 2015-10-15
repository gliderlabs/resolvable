package resolver

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
)

type Resolver interface {
	AddHost(id string, addr net.IP, name string, aliases ...string) error
	RemoveHost(id string) error

	AddUpstream(id string, addr net.IP, port int, domain ...string) error
	RemoveUpstream(id string) error

	Listen() error
	Close()
}

type hostsEntry struct {
	Address net.IP
	Names   []string
}

type serversEntry struct {
	Address net.IP
	Port    int
	Domains []string
}

type dnsResolver struct {
	hostMutex     sync.RWMutex
	upstreamMutex sync.RWMutex

	Port     int
	hosts    map[string]*hostsEntry
	upstream map[string]*serversEntry
	server   *dns.Server
	stopped  chan struct{}
}

func NewResolver() (*dnsResolver, error) {
	return &dnsResolver{
		Port:     53,
		hosts:    make(map[string]*hostsEntry),
		upstream: make(map[string]*serversEntry),
		stopped:  make(chan struct{}),
	}, nil
}

func (r *dnsResolver) AddHost(id string, addr net.IP, name string, aliases ...string) error {
	r.hostMutex.Lock()
	defer r.hostMutex.Unlock()

	r.hosts[id] = &hostsEntry{Address: addr, Names: append([]string{name}, aliases...)}
	return nil
}

func (r *dnsResolver) RemoveHost(id string) error {
	r.hostMutex.Lock()
	defer r.hostMutex.Unlock()

	delete(r.hosts, id)
	return nil
}

func (r *dnsResolver) AddUpstream(id string, addr net.IP, port int, domains ...string) error {
	r.upstreamMutex.Lock()
	defer r.upstreamMutex.Unlock()

	r.upstream[id] = &serversEntry{Address: addr, Port: port, Domains: domains}
	return nil
}

func (r *dnsResolver) RemoveUpstream(id string) error {
	r.upstreamMutex.Lock()
	defer r.upstreamMutex.Unlock()

	delete(r.upstream, id)
	return nil
}

func (r *dnsResolver) Listen() error {
	addr := fmt.Sprintf(":%d", r.Port)

	listenAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp4", listenAddr)
	if err != nil {
		return err
	}

	r.Port = conn.LocalAddr().(*net.UDPAddr).Port

	startupError := make(chan error)
	r.server = &dns.Server{Handler: r, PacketConn: conn, NotifyStartedFunc: func() {
		startupError <- nil
	}}

	go func() {
		select {
		case startupError <- r.run():
		default:
		}
	}()

	return <-startupError
}

func (r *dnsResolver) run() error {
	defer close(r.stopped)
	return r.server.ActivateAndServe()
}

func (r *dnsResolver) Wait() error {
	<-r.stopped
	return nil
}

func (r *dnsResolver) Close() {
	if r.server != nil {
		r.server.Shutdown()
	}
}

func isUDP(w dns.ResponseWriter) bool {
	return strings.HasPrefix(w.RemoteAddr().Network(), "udp")
}

func truncate(m *dns.Msg, udp bool) *dns.Msg {
	m.Truncated = udp && m.Len() > dns.MinMsgSize
	return m
}

func (r *dnsResolver) ServeDNS(w dns.ResponseWriter, query *dns.Msg) {
	response, err := r.responseForQuery(query)
	if err != nil {
		log.Printf("response error: %T %s", err, err)
		return
	}
	if response == nil {
		return
	}

	response.Compress = true

	err = w.WriteMsg(truncate(response, isUDP(w)))
	if err != nil {
		log.Println("write error:", err)
	}
}

func (r *dnsResolver) responseForQuery(query *dns.Msg) (*dns.Msg, error) {
	// TODO multiple queries?
	name := query.Question[0].Name

	if query.Question[0].Qtype == dns.TypeA {
		if addrs := r.findHost(name); len(addrs) > 0 {
			return dnsAddressRecord(query, name, addrs), nil
		}
	} else if query.Question[0].Qtype == dns.TypePTR {
		if hosts := r.findReverse(name); len(hosts) > 0 {
			return dnsPtrRecord(query, name, hosts), nil
		}
	}

	// What if RecursionDesired = false?
	if resp, err := r.findUpstream(name, query); resp != nil || err != nil {
		return resp, err
	}

	return dnsNotFound(query), nil
}

func (r *dnsResolver) upstreamForHost(name string) (matchedUpstream *serversEntry) {
	r.upstreamMutex.RLock()
	defer r.upstreamMutex.RUnlock()

	matchedDomain := ""

	for _, upstream := range r.upstream {
		if len(upstream.Domains) == 0 && matchedDomain == "" {
			matchedUpstream = upstream
		}

		for _, domain := range upstream.Domains {
			domain = dns.Fqdn(domain)
			if len(domain) > len(matchedDomain) && (domain == name || strings.HasSuffix(name, "."+domain)) {
				matchedDomain = domain
				matchedUpstream = upstream
			}
		}
	}

	return
}

func (r *dnsResolver) findUpstream(name string, msg *dns.Msg) (*dns.Msg, error) {
	upstream := r.upstreamForHost(name)
	if upstream == nil || upstream.Address == nil {
		return nil, nil
	}

	c := &dns.Client{Net: "udp"}
	addr := fmt.Sprintf("%s:%d", upstream.Address.String(), upstream.Port)
	resp, _, err := c.Exchange(msg, addr)
	return resp, err
}

func (r *dnsResolver) findHost(name string) (addrs []net.IP) {
	r.hostMutex.RLock()
	defer r.hostMutex.RUnlock()

	for _, hosts := range r.hosts {
		for _, hostName := range hosts.Names {
			if dns.Fqdn(hostName) == name {
				addrs = append(addrs, hosts.Address)
			}
		}
	}
	return
}

func (r *dnsResolver) findReverse(address string) (hosts []string) {
	r.hostMutex.RLock()
	defer r.hostMutex.RUnlock()

	address = strings.ToLower(dns.Fqdn(address))

	for _, entry := range r.hosts {
		if r, _ := dns.ReverseAddr(entry.Address.String()); address == r && len(entry.Names) > 0 {
			hosts = append(hosts, dns.Fqdn(entry.Names[0]))
		}
	}
	return
}

func dnsAddressRecord(query *dns.Msg, name string, addrs []net.IP) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(query)
	for _, addr := range addrs {
		rr := new(dns.A)
		rr.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 0}
		rr.A = addr

		resp.Answer = append(resp.Answer, rr)
	}
	return resp
}

func dnsPtrRecord(query *dns.Msg, name string, hosts []string) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(query)
	for _, host := range hosts {
		rr := new(dns.PTR)
		rr.Hdr = dns.RR_Header{Name: name, Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: 0}
		rr.Ptr = host

		resp.Answer = append(resp.Answer, rr)
	}
	return resp
}

func dnsNotFound(query *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(query)
	resp.SetRcode(query, dns.RcodeNameError)
	return resp
}
