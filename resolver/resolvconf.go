package resolver

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
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

	if _, err = f.Write(orig); err != nil {
		return err
	}

	// contents may have been shortened, so truncate where we are
	pos, err := f.Seek(0, os.SEEK_CUR)
	if err != nil {
		return err
	}
	return f.Truncate(pos)
}
