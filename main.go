package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"runtime"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

func main() {
	// Lock the OS Thread: Namespaces are thread-local in Linux
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if os.Geteuid() != 0 {
		log.Fatal("Error: This program must be run as root.")
	}

	// 1. Create Namespaces
	ns1, _ := netns.NewNamed("ns1")
	ns2, _ := netns.NewNamed("ns2")
	routerNs, _ := netns.NewNamed("router-ns")
	rootNs, _ := netns.Get()
	defer rootNs.Close()

	fmt.Println("✅ Namespaces Created: ns1, ns2, router-ns")

	// 2. Setup Bridges in Root Namespace
	setupBridge := func(name string) *netlink.Bridge {
		br := &netlink.Bridge{LinkAttrs: netlink.LinkAttrs{Name: name}}
		if err := netlink.LinkAdd(br); err != nil {
			log.Fatalf("Failed to create bridge %s: %v", name, err)
		}
		netlink.LinkSetUp(br)
		return br
	}
	br0 := setupBridge("br0")
	br1 := setupBridge("br1")

	// 3. VETH Connection Function
	// This creates a pair, moves one end to a namespace, and attaches the other to a bridge
	connectToBridge := func(vethName, peerName string, targetNs netns.NsHandle, bridge *netlink.Bridge) {
		veth := &netlink.Veth{
			LinkAttrs: netlink.LinkAttrs{Name: vethName},
			PeerName:  peerName,
		}
		netlink.LinkAdd(veth)
		
		// Move Veth to Namespace
		peer, _ := netlink.LinkByName(vethName)
		netlink.LinkSetNsFd(peer, int(targetNs))
		
		// Attach Peer to Bridge in Root NS
		brEnd, _ := netlink.LinkByName(peerName)
		netlink.LinkSetMaster(brEnd, bridge)
		netlink.LinkSetUp(brEnd)
	}

	connectToBridge("veth-ns1", "veth-br0", ns1, br0)
	connectToBridge("veth-ns2", "veth-br1", ns2, br1)
	connectToBridge("veth-r0", "veth-br0-r", routerNs, br0)
	connectToBridge("veth-r1", "veth-br1-r", routerNs, br1)

	// 4. Configure IPs and Routes
	// Helper to jump into a namespace and set IP/GW
	configureInterface := func(ns netns.NsHandle, ifaceName, cidr, gateway string) {
		netns.Set(ns)
		link, _ := netlink.LinkByName(ifaceName)
		addr, _ := netlink.ParseAddr(cidr)
		netlink.AddrAdd(link, addr)
		netlink.LinkSetUp(link)
		
		if gateway != "" {
			gw := net.ParseIP(gateway)
			netlink.RouteAdd(&netlink.Route{
				Scope: netlink.SCOPE_UNIVERSE,
				Gw:    gw,
			})
		}
		netns.Set(rootNs)
	}

	configureInterface(ns1, "veth-ns1", "10.0.1.10/24", "10.0.1.1")
	configureInterface(ns2, "veth-ns2", "10.0.2.10/24", "10.0.2.1")
	
	// Router Interface Setup
	netns.Set(routerNs)
	r0, _ := netlink.LinkByName("veth-r0")
	r1, _ := netlink.LinkByName("veth-r1")
	ra1, _ := netlink.ParseAddr("10.0.1.1/24")
	ra2, _ := netlink.ParseAddr("10.0.2.1/24")
	netlink.AddrAdd(r0, ra1)
	netlink.AddrAdd(r1, ra2)
	netlink.LinkSetUp(r0)
	netlink.LinkSetUp(r1)
	
	// Enable IP Forwarding inside router-ns
	os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644)
	netns.Set(rootNs)

	fmt.Println("🚀 Network Simulation is live!")
}