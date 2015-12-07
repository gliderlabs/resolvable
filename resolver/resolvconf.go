package resolver

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/gliderlabs/resolvable/upstart"
)

const RESOLVCONF_COMMENT = "# added by resolvable"

var resolvConfPattern = regexp.MustCompile("(?m:^.*" + regexp.QuoteMeta(RESOLVCONF_COMMENT) + ")(?:$|\n)")

type ResolvConf struct {
	path string
}

func init() {
	resolveConf := getopt("RESOLV_CONF", "/tmp/resolv.conf")
	HostResolverConfigs.Register(&ResolvConf{resolveConf}, "resolvconf")
}

func (r *ResolvConf) StoreAddress(address string) error {
	resolveConfEntry := fmt.Sprintf("nameserver %s %s\n", address, RESOLVCONF_COMMENT)
	return updateResolvConf(resolveConfEntry, r.path)
}

func (r *ResolvConf) Clean() {
	updateResolvConf("", r.path)
}

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func updateResolvConf(insert, path string) error {
	log.Println("updating resolv.conf:", path)

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	orig, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	orig = resolvConfPattern.ReplaceAllLiteral(orig, []byte{})

	if _, err = f.Seek(0, os.SEEK_SET); err != nil {
		return err
	}

	if _, err = f.WriteString(insert); err != nil {
		return err
	}

	lines := strings.SplitAfter(string(orig), "\n")
	for _, line := range lines {
		// if file ends in a newline, skip empty string from splitting
		if line == "" {
			continue
		}
		if insert == "" {
			line = strings.TrimLeft(line, "# ")
		} else {
			line = "# " + line
		}
		if _, err = f.WriteString(line); err != nil {
			return err
		}
	}

	// contents may have been shortened, so truncate where we are
	pos, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}

	err2 := reload("resolvconf")
	if err2 != nil {
		return err2
	}

	return f.Truncate(pos)
}

func reload(name string) error {
	upstartDbusPath := getopt("UPSTART_DBUS_PATH", "/var/run/dbus/system_bus_socket")
	if _, err := os.Stat(upstartDbusPath); err != nil {
		log.Printf("upstart: disabled, cannot read %s: %s", upstartDbusPath, err)
		return nil
	}

	log.Printf("upstart: %s: starting reload...", name)

	conn, err := upstart.Dial()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to connect to session bus:", err)
		os.Exit(1)
	}

	j, err := conn.Job(name)
	if err != nil {
		return err
	}

	err2 := j.Restart()
	if err2 != nil {
		return err2
	}

	return nil
}
