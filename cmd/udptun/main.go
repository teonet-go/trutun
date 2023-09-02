// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Teonet simple UDP tunnel client/server application. Creates regular tunnel
// between hosts.
package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"github.com/teonet-go/tru"
	"github.com/teonet-go/tru/teolog"
)

const (
	appShor    = "udptun"
	appName    = "Tunnel application"
	appVersion = "0.0.1"
)

var name = flag.String("name", appShor, "interface name")
var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")
var loglevel = flag.String("loglevel", "connect", "set log level")
var logfilter = flag.String("logfilter", "", "set log filter")
var stat = flag.Bool("stat", false, "print statistic")
var hotkey = flag.Bool("hotkey", false, "start hotkey menu")
var postcon = flag.String("pc", "", "post connection commands")
var datalen = flag.Int("datalen", 757, "set max data len in created packets, 0 - maximum UDP len")

var log = teolog.New()

func main() {
	// Print logo message
	fmt.Println(tru.Logo(appName, appVersion))

	// Parse flags
	flag.Parse()
	if len(*name) == 0 {
		flag.Usage()
		return
	}

	log.SetLevel(loglevel)

	// Start UDP tunnel
	_, err := NewUdpTun()
	if err != nil {
		panic(err.Error())
	}
	select {}
}

type UdpTun struct {
	conn net.PacketConn
	ifce *water.Interface
	addr *net.Addr
}

// Create new UDP tunnel
func NewUdpTun() (t *UdpTun, err error) {

	t = new(UdpTun)

	// Create tap interface
	t.ifce, err = t.Interface(*name)
	if err != nil {
		err = errors.New("can't create interface, error: " + err.Error())
		return
	}

	// Create udp connection
	t.conn, err = t.Udp(
		*port, tru.Stat(*stat), tru.Hotkey(*hotkey), log, *loglevel,
		teolog.Logfilter(*logfilter), tru.MaxDataLenType(*datalen),
	)
	if err != nil {
		err = errors.New("can't create tru, error: " + err.Error())
		return
	}

	// Exec post connection commands
	t.PostConnect(*postcon)

	return
}

// Udp create new udp connection
func (t *UdpTun) Udp(port int, params ...interface{}) (conn net.PacketConn, err error) {

	// Create server connection and start listen incominng packets
	// con, err = tru.New(port, append(params,
	// 	// Tru reader get all packets and resend it to interface
	// 	func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
	// 		if err != nil {
	// 			log.Debug.Println("got error in main reader:", err)
	// 			return
	// 		}
	// 		log.Debug.Printf("got %d byte from %s, id %d, len %d\n",
	// 			pac.Len(), ch.Addr().String(), pac.ID(), len(pac.Data()))

	// 		// TODO: wait ifce ready
	// 		for t.ifce == nil {
	// 			time.Sleep(10 * time.Millisecond)
	// 		}
	// 		t.ifce.Write(pac.Data())
	// 		return
	// 	},
	// )...)
	// if err != nil {
	// 	log.Error.Fatal("can't create tru, err: ", err)
	// }

	// Start listen udp port
	conn, err = net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return
	}
	// defer udpServer.Close()

	// Set address for client mode
	if len(*addr) > 0 {
		fmt.Printf("udp client mode\n")
		var udpAddr *net.UDPAddr
		udpAddr, err = net.ResolveUDPAddr("udp", *addr)
		if err != nil {
			return
		}
		var a net.Addr = udpAddr
		t.addr = &a
	} else {
		fmt.Printf("udp server mode\n")
	}

	// Udp packets reader
	go func() {
		// TODO: wait ifce ready
		fmt.Printf("wait for ifce\n")
		for t.ifce == nil {
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Printf("ifce done %v\n", t.ifce)

		buf := make([]byte, 2*1024)
		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}
			fmt.Printf("got %d byte from %s\n", n, addr)

			t.ifce.Write(buf[:n])
			if t.addr == nil {
				t.addr = &addr
			}
		}
	}()

	// Connect to peer
	// go func() {
	// 	var reconnect = make(chan interface{})
	// 	defer close(reconnect)
	// 	for {
	// 		if _, err := con.Connect(*addr, func(ch *tru.Channel, pac *tru.Packet,
	// 			err error) (processed bool) {
	// 			if err != nil {
	// 				reconnect <- nil
	// 			}
	// 			return false
	// 		}); err == nil {
	// 			<-reconnect
	// 		}
	// 		log.Connect.Println("reconnect to", *addr)
	// 	}
	// }()

	return
}

// Interface create tap interface
func (t *UdpTun) Interface(name string) (ifce *water.Interface, err error) {
	config := water.Config{
		DeviceType: water.TAP,
	}
	config.Name = name

	// Create interface
	ifce, err = water.New(config)
	if err != nil {
		return
	}

	// Read from interface and send to tru channels
	go func() {

		// TODO: wait udp ready
		fmt.Printf("wait for address\n")
		for t.addr == nil {
			time.Sleep(10 * time.Millisecond)
		}
		fmt.Printf("address connected %s\n", *t.addr)

		// Create frame
		var frame ethernet.Frame
		frame.Resize(1500)

		// Read iface and resend frames to Udp
		for {
			// frame.Resize(1500)
			n, err := ifce.Read([]byte(frame))
			if err != nil {
				log.Error.Fatal(err)
			}
			// frame = frame[:n]
			log.Debug.Printf("Dst: %s\n", frame.Destination())
			log.Debug.Printf("Src: %s\n", frame.Source())
			log.Debug.Printf("Ethertype: % x\n", frame.Ethertype())
			log.Debug.Printf("Payload len: %d\n", len(frame.Payload()))
			// log.Debugvv.Printf("Payload: % x\n", frame.Payload())

			// Resend frame to all channels
			// t.udp.ForEachChannel(func(ch *tru.Channel) { ch.WriteTo(frame[:n]) })
			// t.udp.WriteTo(frame[:n], t.addr)
			// t.Write(frame[:n])

			n, err = t.conn.WriteTo(frame[:n], *t.addr)
			if err != nil {
				fmt.Printf("write to %s error: %s\n", *t.addr, err)
				continue
			}
			fmt.Printf("write %d byte to %s\n", n, *t.addr)
		}
	}()

	return
}

// PostConnect execute post connection os commands
func (t *UdpTun) PostConnect(commands string) {
	if len(commands) == 0 {
		return
	}
	com := strings.Split(commands, " ")
	var arg []string
	if len(com) > 1 {
		arg = com[1:]
	}

	out, err := exec.Command(com[0], arg...).Output()
	if err != nil {
		log.Error.Println("can't execute post connection commands, err: ", err)
	}
	log.Debug.Printf("\n%s\n", out)
}
