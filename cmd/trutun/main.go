// Copyright 2022 Kirill Scherba <kirill@scherba.ru>. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Teonet TRU tunnel client/server application. Creates regular tunnel between
// hosts.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
	"github.com/teonet-go/tru"
	"github.com/teonet-go/tru/teolog"
)

const (
	appShor    = "trutun"
	appName    = "Tunnel application"
	appVersion = "0.0.9"
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

	// Start TRU tunnel
	_, err := NewTruTun()
	if err != nil {
		panic(err.Error())
	}
	select {}
}

type TruTun struct {
	tru  *tru.Tru
	ifce *water.Interface
}

// Create new TRU tunnel
func NewTruTun() (t *TruTun, err error) {

	t = new(TruTun)

	// Create tap interface
	t.ifce, err = t.Interface(*name)
	if err != nil {
		err = errors.New("can't create interface, error: " + err.Error())
		return
	}

	// Connect to tru
	t.tru, err = t.Tru(
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

// Tru create new tru connection
func (t *TruTun) Tru(port int, params ...interface{}) (con *tru.Tru, err error) {

	// Create server connection and start listen incominng packets
	con, err = tru.New(port, append(params,
		// Tru reader get all packets and resend it to interface
		func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
			if err != nil {
				log.Debug.Println("got error in main reader:", err)
				return
			}
			log.Debug.Printf("got %d byte from %s, id %d, len %d\n",
				pac.Len(), ch.Addr().String(), pac.ID(), len(pac.Data()))

			// TODO: wait ifce ready
			for t.ifce == nil {
				time.Sleep(10 * time.Millisecond)
			}
			t.ifce.Write(pac.Data())
			return
		},
	)...)
	if err != nil {
		log.Error.Fatal("can't create tru, err: ", err)
	}

	// Return if server mode
	if len(*addr) == 0 {
		return
	}

	// Connect to peer
	go func() {
		var reconnect = make(chan interface{})
		defer close(reconnect)
		for {
			if _, err := con.Connect(*addr, func(ch *tru.Channel, pac *tru.Packet,
				err error) (processed bool) {
				if err != nil {
					reconnect <- nil
				}
				return false
			}); err == nil {
				<-reconnect
			}
			log.Connect.Println("reconnect to", *addr)
		}
	}()

	return
}

// Interface create tap interface
func (t *TruTun) Interface(name string) (ifce *water.Interface, err error) {
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
		var frame ethernet.Frame
		frame.Resize(1500 - 14)
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
			// TODO: wait tru ready
			for t.tru == nil {
				time.Sleep(10 * time.Millisecond)
			}
			t.tru.ForEachChannel(func(ch *tru.Channel) { ch.WriteTo(frame[:n]) })
		}
	}()

	return
}

// PostConnect execute post connection os commands
func (t *TruTun) PostConnect(commands string) {
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
