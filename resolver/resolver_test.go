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

func TestDnsmasqResolver(t *testing.T) {
	resolver, err := NewDnsmasqResolver()
	ok(t, err)
	resolver.Port = 5388

	hostname := "foobar"
	address := net.ParseIP("1.2.3.4")

	resolver.AddHost(address.String(), address, hostname)

	ok(t, resolver.Listen())
	defer resolver.Close()

	// TODO should wait until the dnsmasq server indicates that it's started
	time.Sleep(time.Second)

	addrs, err := lookupHost(hostname, "127.0.0.1:5388")
	ok(t, err)
	equals(t, []net.IP{address}, addrs)

	resolver.RemoveHost(address.String())

	addrs, err = lookupHost(hostname, "127.0.0.1:5388")
	ok(t, err)
	equals(t, []net.IP{}, addrs)
}

func TestUpstreamResolver(t *testing.T) {
	resolver, err := NewDnsmasqResolver()
	ok(t, err)
	resolver.Port = 5388

	hostname := "foobar"
	address := net.ParseIP("1.2.3.4")

	ok(t, resolver.Listen())
	defer resolver.Close()

	// TODO should wait until the dnsmasq server indicates that it's started
	time.Sleep(time.Second)

	// host should be missing initially
	addrs, err := lookupHost(hostname, "127.0.0.1:5388")
	ok(t, err)
	equals(t, []net.IP{}, addrs)

	upstream, err := NewDnsmasqResolver()
	ok(t, err)
	upstream.Port = 5389

	ok(t, upstream.Listen())
	defer upstream.Close()
	time.Sleep(time.Second)

	upstream.AddHost("foobar", address, hostname)

	resolver.AddUpstream("foobar", net.ParseIP("127.0.0.1"), upstream.Port)

	// should be able to resolve now
	addrs, err = lookupHost(hostname, "127.0.0.1:5388")
	ok(t, err)
	equals(t, []net.IP{address}, addrs)

}
