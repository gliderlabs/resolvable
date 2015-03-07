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

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
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
	resolver := NewDnsmasqResolver()
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
