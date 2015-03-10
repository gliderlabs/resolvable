package resolver

import (
	"fmt"
	"net"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/tonnerre/golang-dns"
)

func TestDnsmasqResolver(t *testing.T) {
	hostname := "foobar"
	address := net.ParseIP("1.2.3.4")

	resolver, err := NewDnsmasqResolver()
	ok(t, err)

	resolver.AddHost(address.String(), address, hostname)

	ok(t, startResolver(resolver, 5388))
	defer resolver.Close()

	assertResolvesTo(t, []net.IP{address}, hostname, resolver.Port)

	resolver.RemoveHost(address.String())
	assertDoesNotResolve(t, hostname, resolver.Port)
}

func TestUpstreamResolver(t *testing.T) {
	hostname := "foobar"
	address := net.ParseIP("1.2.3.4")

	resolver, err := runResolver(5388)
	ok(t, err)
	defer resolver.Close()

	upstream, err := runResolver(5389)
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

	resolver, err := runResolver(5388)
	ok(t, err)
	defer resolver.Close()

	upstream, err := runResolver(5389)
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

// queries within the "LocalDomain" should not be forwarded to upstream servers
func TestLocalDomain(t *testing.T) {
	shouldResolve := net.ParseIP("1.0.0.1")
	shouldNotResolve := net.ParseIP("3.0.0.1")

	resolver, err := NewDnsmasqResolver()
	ok(t, err)

	resolver.LocalDomain = "docker"

	ok(t, startResolver(resolver, 5388))
	defer resolver.Close()

	upstream, err := runResolver(5389)
	ok(t, err)
	defer upstream.Close()

	resolver.AddUpstream("upstream", net.ParseIP("127.0.0.1"), upstream.Port)
	resolver.AddHost("should-resolve", shouldResolve, "should-resolve.docker")
	upstream.AddHost("should-not-resolve", shouldNotResolve, "should-not-resolve.docker")

	assertDoesNotResolve(t, "should-not-resolve.docker", resolver.Port)
	assertResolvesTo(t, []net.IP{shouldResolve}, "should-resolve.docker", resolver.Port)
}

func TestWait(t *testing.T) {
	resolver, err := runResolver(5388)
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
	case <-time.After(time.Second / 2):
		t.Fatal("wait should return after process has exited")
	}
}

func TestWaitBeforeListen(t *testing.T) {
	resolver, err := NewDnsmasqResolver()
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

	ok(t, startResolver(resolver, 5388))

	select {
	case <-done:
		t.Fatal("wait should not return before process has exited")
	case <-time.After(time.Second / 2):
	}

	resolver.Close()

	select {
	case <-done:
	case <-time.After(time.Second / 2):
		t.Fatal("wait should return after process has exited")
	}
}

////////////////////////////////////////////////////////////////////////////////

func startResolver(resolver *dnsmasqResolver, port int) error {
	resolver.Port = port
	if err := resolver.Listen(); err != nil {
		return err
	}
	// FIXME should wait just until the dnsmasq server indicates that it's started
	time.Sleep(time.Second)
	return nil
}

func runResolver(port int) (*dnsmasqResolver, error) {
	resolver, err := NewDnsmasqResolver()
	if err == nil {
		err = startResolver(resolver, port)
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
	equals(tb, expected, addrs)
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
