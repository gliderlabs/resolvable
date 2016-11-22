package systemd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/gliderlabs/resolvable/resolver"

	"github.com/coreos/go-systemd/daemon"
	"github.com/coreos/go-systemd/dbus"
)

type service struct {
	name        string
	dir         string
	filepattern string
}

var defaultServices []service = []service{
	service{"systemd-resolved.service", "resolved.conf.d", "*.conf"},
	service{"systemd-networkd.service", "network", "*.network"},
}

type SystemdConfig struct {
	templatePath string
	destPath     string
	services     []service
	written      map[string][]string
}

type templateArgs struct {
	Address string
}

func getopt(name, def string) string {
	if env := os.Getenv(name); env != "" {
		return env
	}
	return def
}

func init() {
	systemdConf := getopt("SYSTEMD_CONF_PATH", "/tmp/systemd")
	if _, err := os.Stat(systemdConf); err != nil {
		log.Printf("systemd: disabled, cannot read %s: %s", systemdConf, err)
		return
	}
	resolver.HostResolverConfigs.Register(&SystemdConfig{
		templatePath: "/config/systemd",
		destPath:     systemdConf,
		services:     defaultServices,
		written:      make(map[string][]string),
	}, "systemd")
}

func (r *SystemdConfig) StoreAddress(address string) error {
	data := templateArgs{address}

	for _, s := range r.services {
		pattern := filepath.Join(r.templatePath, s.dir, s.filepattern)

		log.Printf("systemd: %s: loading config from %s", s.name, pattern)

		templates, err := template.ParseGlob(pattern)
		if err != nil {
			log.Println("systemd:", err)
			continue
		}

		var written []string

		for _, t := range templates.Templates() {
			dest := filepath.Join(r.destPath, s.dir, t.Name())
			log.Println("systemd: generating", dest)
			fp, err := os.Create(dest)
			if err != nil {
				log.Println("systemd:", err)
				continue
			}
			written = append(written, dest)
			t.Execute(fp, data)
			fp.Close()
		}

		if written != nil {
			r.written[s.name] = written
			reload(s.name)
		} else {
			log.Println("systemd: %s: no configs written, skipping reload", s.name)
		}
	}

	daemon.SdNotify(false, "READY=1")
	return nil
}

func (r *SystemdConfig) Clean() {
	daemon.SdNotify(false, "STOPPING=1")

	for service, filenames := range r.written {
		log.Printf("systemd: %s: removing configs...", service)
		for _, filename := range filenames {
			os.Remove(filename)
		}
		reload(service)
	}
}

func reload(name string) error {
	conn, err := dbus.New()
	if err != nil {
		return err
	}

	log.Printf("systemd: %s: starting reload...", name)

	statusCh := make(chan string)
	_, err = conn.ReloadOrRestartUnit(name, "replace", statusCh)
	if err != nil {
		return err
	}

	status := <-statusCh
	log.Printf("systemd: %s: %s", name, status)
	if status != "done" {
		return fmt.Errorf("error reloading %s: %s", name, status)
	}
	return nil
}
