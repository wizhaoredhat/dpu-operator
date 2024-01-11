package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/vishvananda/netlink"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ipam"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/wizhaoredhat/dpu-operator/pkg/logging"
)

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

type NetConf struct {
	types.NetConf
	LogLevel string `json:"logLevel,omitempty"`
	LogFile  string `json:"logFile,omitempty"`
}

func setLogging(stdinData []byte, containerID, netns, ifName string) error {
	n := &NetConf{}
	if err := json.Unmarshal(stdinData, n); err != nil {
		return fmt.Errorf("setLogging(): failed to load netconf: %v", err)
	}

	logging.Init(n.LogLevel, n.LogFile, containerID, netns, ifName)
	return nil
}

func createDummy(ifName string, netns ns.NetNS) (*current.Interface, error) {
	dummy := &current.Interface{}

	dm := &netlink.Dummy{
		LinkAttrs: netlink.LinkAttrs{
			Name:      ifName,
			Namespace: netlink.NsFd(int(netns.Fd())),
		},
	}

	if err := netlink.LinkAdd(dm); err != nil {
		return nil, fmt.Errorf("failed to create dummy: %v", err)
	}
	dummy.Name = ifName

	err := netns.Do(func(_ ns.NetNS) error {
		// Re-fetch interface to get all properties/attributes
		contDummy, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to fetch dummy%q: %v", ifName, err)
		}

		dummy.Mac = contDummy.Attrs().HardwareAddr.String()
		dummy.Sandbox = netns.Path()

		return nil
	})
	if err != nil {
		return nil, err
	}

	return dummy, nil
}

func parseNetConf(bytes []byte) (*types.NetConf, error) {
	fp, _ := os.OpenFile("/tmp/cni_debug", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	defer fp.Close()

	conf := &types.NetConf{}
	if err := json.Unmarshal(bytes, conf); err != nil {
		return nil, fmt.Errorf("failed to parse network config: %v", err)
	}

	fmt.Fprintf(fp, "conf = %x", conf)

	if conf.RawPrevResult != nil {
		if err := version.ParsePrevResult(conf); err != nil {
			return nil, fmt.Errorf("failed to parse prevResult: %v", err)
		}
		if _, err := current.NewResultFromResult(conf.PrevResult); err != nil {
			return nil, fmt.Errorf("failed to convert result to current version: %v", err)
		}
	}

	return conf, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	if err := setLogging(args.StdinData, args.ContainerID, args.Netns, args.IfName); err != nil {
		return err
	}

	logging.Info("function called",
		"func", "cmdAdd",
		"args.Path", args.Path, "args.StdinData", string(args.StdinData), "args.Args", args.Args)

	conf, err := parseNetConf(args.StdinData)
	if err != nil {
		return err
	}

	if conf.IPAM.Type == "" {
		return errors.New("dummy interface requires an IPAM configuration")
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", netns, err)
	}
	defer netns.Close()

	dummyInterface, err := createDummy(args.IfName, netns)
	if err != nil {
		return err
	}

	// Delete link if err to avoid link leak in this ns
	defer func() {
		if err != nil {
			netns.Do(func(_ ns.NetNS) error {
				return ip.DelLinkByName(args.IfName)
			})
		}
	}()

	r, err := ipam.ExecAdd(conf.IPAM.Type, args.StdinData)
	if err != nil {
		return err
	}

	// defer ipam deletion to avoid ip leak
	defer func() {
		if err != nil {
			ipam.ExecDel(conf.IPAM.Type, args.StdinData)
		}
	}()

	// convert IPAMResult to current Result type
	result, err := current.NewResultFromResult(r)
	if err != nil {
		return err
	}

	if len(result.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}

	for _, ipc := range result.IPs {
		// all addresses apply to the container dummy interface
		ipc.Interface = current.Int(0)
	}

	result.Interfaces = []*current.Interface{dummyInterface}

	err = netns.Do(func(_ ns.NetNS) error {
		return ipam.ConfigureIface(args.IfName, result)
	})

	if err != nil {
		return err
	}

	return types.PrintResult(result, conf.CNIVersion)
}

func cmdDel(args *skel.CmdArgs) error {
	if err := setLogging(args.StdinData, args.ContainerID, args.Netns, args.IfName); err != nil {
		return err
	}
	logging.Info("function called",
		"func", "cmdDel",
		"args.Path", args.Path, "args.StdinData", string(args.StdinData), "args.Args", args.Args)

	conf, err := parseNetConf(args.StdinData)
	if err != nil {
		return err
	}

	if err = ipam.ExecDel(conf.IPAM.Type, args.StdinData); err != nil {
		return err
	}

	if args.Netns == "" {
		return nil
	}

	err = ns.WithNetNSPath(args.Netns, func(ns.NetNS) error {
		err = ip.DelLinkByName(args.IfName)
		if err != nil && err == ip.ErrLinkNotFound {
			return nil
		}
		return err
	})

	if err != nil {
		//  if NetNs is passed down by the Cloud Orchestration Engine, or if it called multiple times
		// so don't return an error if the device is already removed.
		// https://github.com/kubernetes/kubernetes/issues/43014#issuecomment-287164444
		_, ok := err.(ns.NSPathNotExistErr)
		if ok {
			return nil
		}
		return err
	}

	return nil
}

func cmdCheck(_ *skel.CmdArgs) error {
	return nil
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, "")
}
