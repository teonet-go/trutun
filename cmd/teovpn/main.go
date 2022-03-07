// TRU vpn execute and configuration example
//
// Create vpn tunnel between two hosts
//
// Server:
//   TRU=tru1 && sudo go run ./cmd/teovpn -name=$TRU -p=9000 -loglevel=Debug -stat -hotkey
//   TRU=tru1 && sudo ip addr add 10.1.1.10/24 dev $TRU && sudo ip link set up dev $TRU
//
// Client:
//   TRU=tru2 && sudo go run ./cmd/teovpn -name=$TRU -a=host.name:9000 -loglevel=Debug -stat -hotkey
//   TRU=tru2 && sudo ip addr add 10.1.1.11/24 dev $TRU && sudo ip link set up dev $TRU
//

package main

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/kirill-scherba/tru"
	"github.com/kirill-scherba/tru/teolog"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"
)

var name = flag.String("name", "", "interface name")
var port = flag.Int("p", 0, "local port number")
var addr = flag.String("a", "", "remote address to connect to")
var loglevel = flag.String("loglevel", "", "set log level")
var logfilter = flag.String("logfilter", "", "set log filter")
var stat = flag.Bool("stat", false, "print statistic")
var hotkey = flag.Bool("hotkey", false, "start hotkey menu")

var log = teolog.New()

func main() {
	// Print logo message
	fmt.Println("TRU based vpn application ver. 0.0.1")

	// Parse flags
	flag.Parse()
	if len(*name) == 0 {
		panic("set name parameter")
	}

	_, err := NewTeoVpn()
	if err != nil {
		panic(err.Error())
	}
	select {}
}

type TeoVpn struct {
	tru  *tru.Tru
	ifce *water.Interface
}

func NewTeoVpn() (tv *TeoVpn, err error) {

	tv = new(TeoVpn)

	// Create tap interface
	tv.ifce, err = tv.Interface(*name)
	if err != nil {
		err = errors.New("can't create interface, error: " + err.Error())
		return
	}

	// Connect to tru
	tv.tru, err = tv.Tru(*port, tru.Stat(*stat), tru.Hotkey(*hotkey),
		log, *loglevel, teolog.Logfilter(*logfilter))
	if err != nil {
		err = errors.New("can't create tru, error: " + err.Error())
		return
	}

	return
}

// Tru create new tru connection
func (tv *TeoVpn) Tru(port int, params ...interface{}) (t *tru.Tru, err error) {

	// Create server connection and start listen incominng packets
	t, err = tru.New(port, append(params,
		// Tru reader get all packets and resend it to interface
		func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
			if err != nil {
				log.Debug.Println("got error in main reader:", err)
				return
			}
			log.Debug.Printf("got %d byte from %s, id %d: % x\n",
				pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())

			// TODO: wait ifce ready
			for tv.ifce == nil {
				time.Sleep(10 * time.Millisecond)
			}
			tv.ifce.Write(pac.Data())
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
			if _, err := t.Connect(*addr, func(ch *tru.Channel, pac *tru.Packet,
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

// Interface
func (tv *TeoVpn) Interface(name string) (ifce *water.Interface, err error) {
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
		for {
			frame.Resize(1500)
			n, err := ifce.Read([]byte(frame))
			if err != nil {
				log.Error.Fatal(err)
			}
			frame = frame[:n]
			log.Debug.Printf("Dst: %s\n", frame.Destination())
			log.Debug.Printf("Src: %s\n", frame.Source())
			log.Debug.Printf("Ethertype: % x\n", frame.Ethertype())
			log.Debug.Printf("Payload: % x\n", frame.Payload())

			// Resend frame to all channels
			// TODO: wait tru ready
			for tv.tru == nil {
				time.Sleep(10 * time.Millisecond)
			}
			tv.tru.ForEachChannel(func(ch *tru.Channel) { ch.WriteTo(frame) })
		}
	}()

	return
}
