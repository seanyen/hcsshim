//go:build linux
// +build linux

package hcsv2

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	oci "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"github.com/Microsoft/hcsshim/internal/guest/network"
	"github.com/Microsoft/hcsshim/internal/guestpath"
	"github.com/Microsoft/hcsshim/internal/oc"
)

func getStandaloneRootDir(id string) string {
	return filepath.Join(guestpath.LCOWRootPrefixInUVM, id)
}

func getStandaloneHostnamePath(id string) string {
	return filepath.Join(getStandaloneRootDir(id), "hostname")
}

func getStandaloneHostsPath(id string) string {
	return filepath.Join(getStandaloneRootDir(id), "hosts")
}

func getStandaloneResolvPath(id string) string {
	return filepath.Join(getStandaloneRootDir(id), "resolv.conf")
}

func setupStandaloneContainerSpec(ctx context.Context, id string, spec *oci.Spec) (err error) {
	ctx, span := trace.StartSpan(ctx, "hcsv2::setupStandaloneContainerSpec")
	defer span.End()
	defer func() { oc.SetSpanStatus(span, err) }()
	span.AddAttributes(trace.StringAttribute("cid", id))

	// Generate the standalone root dir
	rootDir := getStandaloneRootDir(id)
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return errors.Wrapf(err, "failed to create container root directory %q", rootDir)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(rootDir)
		}
	}()

	hostname := spec.Hostname
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			return errors.Wrap(err, "failed to get hostname")
		}
	}

	// Write the hostname
	if !isInMounts("/etc/hostname", spec.Mounts) {
		standaloneHostnamePath := getStandaloneHostnamePath(id)
		if err := ioutil.WriteFile(standaloneHostnamePath, []byte(hostname+"\n"), 0644); err != nil {
			return errors.Wrapf(err, "failed to write hostname to %q", standaloneHostnamePath)
		}

		mt := oci.Mount{
			Destination: "/etc/hostname",
			Type:        "bind",
			Source:      getStandaloneHostnamePath(id),
			Options:     []string{"bind"},
		}
		if isRootReadonly(spec) {
			mt.Options = append(mt.Options, "ro")
		}
		spec.Mounts = append(spec.Mounts, mt)
	}

	// Write the hosts
	if !isInMounts("/etc/hosts", spec.Mounts) {
		standaloneHostsContent := network.GenerateEtcHostsContent(ctx, hostname)
		standaloneHostsPath := getStandaloneHostsPath(id)
		if err := ioutil.WriteFile(standaloneHostsPath, []byte(standaloneHostsContent), 0644); err != nil {
			return errors.Wrapf(err, "failed to write standalone hosts to %q", standaloneHostsPath)
		}

		mt := oci.Mount{
			Destination: "/etc/hosts",
			Type:        "bind",
			Source:      getStandaloneHostsPath(id),
			Options:     []string{"bind"},
		}
		if isRootReadonly(spec) {
			mt.Options = append(mt.Options, "ro")
		}
		spec.Mounts = append(spec.Mounts, mt)
	}

	// Write resolv.conf
	if !isInMounts("/etc/resolv.conf", spec.Mounts) {
		ns := getOrAddNetworkNamespace(getNetworkNamespaceID(spec))
		var searches, servers []string
		for _, n := range ns.Adapters() {
			if len(n.DNSSuffix) > 0 {
				searches = network.MergeValues(searches, strings.Split(n.DNSSuffix, ","))
			}
			if len(n.DNSServerList) > 0 {
				servers = network.MergeValues(servers, strings.Split(n.DNSServerList, ","))
			}
		}
		resolvContent, err := network.GenerateResolvConfContent(ctx, searches, servers, nil)
		if err != nil {
			return errors.Wrap(err, "failed to generate standalone resolv.conf content")
		}
		standaloneResolvPath := getStandaloneResolvPath(id)
		if err := ioutil.WriteFile(standaloneResolvPath, []byte(resolvContent), 0644); err != nil {
			return errors.Wrap(err, "failed to write standalone resolv.conf")
		}

		mt := oci.Mount{
			Destination: "/etc/resolv.conf",
			Type:        "bind",
			Source:      getStandaloneResolvPath(id),
			Options:     []string{"bind"},
		}
		if isRootReadonly(spec) {
			mt.Options = append(mt.Options, "ro")
		}
		spec.Mounts = append(spec.Mounts, mt)
	}

	// Force the parent cgroup into our /containers root
	spec.Linux.CgroupsPath = "/containers/" + id

	// Clear the windows section as we dont want to forward to runc
	spec.Windows = nil

	return nil
}
