package resolver

import (
	"fmt"
	"net"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/tonnerre/golang-dns"
)

func TestResolver(t *testing.T) {
	hostname := "foobar"
	address := net.ParseIP("1.2.3.4")

	resolver, err := NewResolver()
	ok(t, err)

	resolver.AddHost(address.String(), address, hostname)

	ok(t, startResolver(resolver))
	defer resolver.Close()

	assertResolvesTo(t, []net.IP{address}, hostname, resolver.Port)

	resolver.RemoveHost(address.String())
	assertDoesNotResolve(t, hostname, resolver.Port)
}

func TestMultipleAddresses(t *testing.T) {
	hostname := "foobar"
	addr1 := net.ParseIP("1.2.3.4")
	addr2 := net.ParseIP("5.6.7.8")

	resolver, err := runResolver()
	ok(t, err)
	defer resolver.Close()

	resolver.AddHost(addr1.String(), addr1, hostname)
	resolver.AddHost(addr2.String(), addr2, hostname)

	assertResolvesTo(t, []net.IP{addr1, addr2}, hostname, resolver.Port)
}

func TestUpstreamResolver(t *testing.T) {
	hostname := "foobar"
	address := net.ParseIP("1.2.3.4")

	resolver, err := runResolver()
	ok(t, err)
	defer resolver.Close()

	upstream, err := runResolver()
	ok(t, err)
	defer upstream.Close()
	upstream.AddHost("foobar", address, hostname)

	assertDoesNotResolve(t, hostname, resolver.Port)

	resolver.AddUpstream("upstream", net.ParseIP("127.0.0.1"), upstream.Port)

	assertResolvesTo(t, []net.IP{address}, hostname, resolver.Port)
}

func TestUpstreamResolverDomains(t *testing.T) {
	shouldResolve := net.ParseIP("1.0.0.1")
	shouldAlsoResolve := net.ParseIP("2.0.0.1")
	shouldNotResolve := net.ParseIP("3.0.0.1")

	resolver, err := runResolver()
	ok(t, err)
	defer resolver.Close()

	upstream, err := runResolver()
	ok(t, err)
	defer upstream.Close()
	upstream.AddHost("should-resolve", shouldResolve, "domain.should-resolve")
	upstream.AddHost("should-also-resolve", shouldAlsoResolve, "domain.should-also-resolve")
	upstream.AddHost("should-not-resolve", shouldNotResolve, "domain.should-not-resolve")

	resolver.AddUpstream("upstream", net.ParseIP("127.0.0.1"), upstream.Port, "should-resolve", "should-also-resolve")

	assertDoesNotResolve(t, "domain.should-not-resolve", resolver.Port)
	assertResolvesTo(t, []net.IP{shouldResolve}, "domain.should-resolve", resolver.Port)
	assertResolvesTo(t, []net.IP{shouldAlsoResolve}, "domain.should-also-resolve", resolver.Port)
}

func TestUpstreamResolverSubDomains(t *testing.T) {
	addr := net.ParseIP("1.0.0.1")

	resolver, err := runResolver()
	ok(t, err)
	defer resolver.Close()

	upstream1, err := runResolver()
	ok(t, err)
	defer upstream1.Close()

	upstream1.AddHost("should-resolve", addr, "name.top")

	upstream2, err := runResolver()
	ok(t, err)
	defer upstream2.Close()

	upstream2.AddHost("should-also-resolve", addr, "name.sub.top")

	resolver.AddUpstream("upstream1", net.ParseIP("127.0.0.1"), upstream1.Port, "top")
	resolver.AddUpstream("upstream2", net.ParseIP("127.0.0.1"), upstream2.Port, "sub.top")

	assertResolvesTo(t, []net.IP{addr}, "name.top", resolver.Port)
	assertResolvesTo(t, []net.IP{addr}, "name.sub.top", resolver.Port)
}

// queries within the "local domain" should not be forwarded to upstream servers
func TestLocalDomain(t *testing.T) {
	shouldResolve := net.ParseIP("1.0.0.1")
	shouldNotResolve := net.ParseIP("3.0.0.1")

	resolver, err := NewResolver()
	ok(t, err)

	// upstream with a "nil" address should not be forwarded
	resolver.AddUpstream("docker", nil, 0, "docker")

	ok(t, startResolver(resolver))
	defer resolver.Close()

	upstream, err := runResolver()
	ok(t, err)
	defer upstream.Close()

	resolver.AddUpstream("upstream", net.ParseIP("127.0.0.1"), upstream.Port)
	resolver.AddHost("should-resolve", shouldResolve, "should-resolve.docker")
	upstream.AddHost("should-not-resolve", shouldNotResolve, "should-not-resolve.docker")

	assertDoesNotResolve(t, "should-not-resolve.docker", resolver.Port)
	assertResolvesTo(t, []net.IP{shouldResolve}, "should-resolve.docker", resolver.Port)
}

func TestReverseLookup(t *testing.T) {
	addr := net.ParseIP("1.2.3.4")

	resolver, err := runResolver()
	ok(t, err)
	defer resolver.Close()

	resolver.AddHost("foo", addr, "primary.domain", "secondary.domain")

	m := new(dns.Msg)
	m.SetQuestion("4.3.2.1.in-addr.arpa.", dns.TypePTR)

	c := new(dns.Client)
	r, _, err := c.Exchange(m, fmt.Sprintf("127.0.0.1:%d", resolver.Port))
	ok(t, err)

	hosts := make([]string, 0, len(r.Answer))
	for _, answer := range r.Answer {
		if record, ok := answer.(*dns.PTR); ok {
			hosts = append(hosts, record.Ptr)
		}
	}

	equals(t, []string{"primary.domain."}, hosts)
}

func TestWait(t *testing.T) {
	resolver, err := runResolver()
	ok(t, err)
	defer resolver.Close()

	done := make(chan struct{})
	go func() {
		resolver.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("wait should not return before process has exited")
	case <-time.After(time.Second / 2):
	}

	resolver.Close()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("wait should return after process has exited")
	}
}

func TestWaitBeforeListen(t *testing.T) {
	resolver, err := NewResolver()
	ok(t, err)
	defer resolver.Close()

	done := make(chan struct{})
	go func() {
		resolver.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("wait should not return before process has exited")
	case <-time.After(time.Second / 2):
	}

	ok(t, startResolver(resolver))

	select {
	case <-done:
		t.Fatal("wait should not return before process has exited")
	case <-time.After(time.Second / 2):
	}

	resolver.Close()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("wait should return after process has exited")
	}
}

////////////////////////////////////////////////////////////////////////////////

func startResolver(resolver *dnsResolver) error {
	resolver.Port = 0
	if err := resolver.Listen(); err != nil {
		return err
	}
	return nil
}

func runResolver() (*dnsResolver, error) {
	resolver, err := NewResolver()
	if err == nil {
		err = startResolver(resolver)
	}
	return resolver, err
}

func lookupHost(host, server string) ([]net.IP, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), dns.TypeA)

	c := new(dns.Client)
	r, _, err := c.Exchange(m, server)

	if err != nil {
		return nil, err
	}

	addrs := make([]net.IP, 0, len(r.Answer))

	for _, answer := range r.Answer {
		if record, ok := answer.(*dns.A); ok {
			addrs = append(addrs, record.A)
		}
	}

	return addrs, nil
}

func assertResolvesTo(tb testing.TB, expected []net.IP, hostname string, dnsPort int) {
	addrs, err := lookupHost(hostname, fmt.Sprintf("127.0.0.1:%d", dnsPort))
	ok(tb, err)
	equals(tb, sortIPs(expected), sortIPs(addrs))
}

func sortIPs(ips []net.IP) []string {
	vals := make([]string, len(ips))
	for i, ip := range ips {
		vals[i] = ip.String()
	}
	sort.Strings(vals)
	return vals
}

func assertDoesNotResolve(tb testing.TB, hostname string, dnsPort int) {
	assertResolvesTo(tb, []net.IP{}, hostname, dnsPort)
}

// ok fails the test if an err is not nil.
func ok(tb testing.TB, err error) {
	if err != nil {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: unexpected error: %s\033[39m\n\n", filepath.Base(file), line, err.Error())
		tb.FailNow()
	}
}

// equals fails the test if exp is not equal to act.
func equals(tb testing.TB, exp, act interface{}) {
	if !reflect.DeepEqual(exp, act) {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d:\n\n\texp: %#v\n\n\tgot: %#v\033[39m\n\n", filepath.Base(file), line, exp, act)
		tb.FailNow()
	}
}
