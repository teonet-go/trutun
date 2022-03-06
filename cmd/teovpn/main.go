// TRU=tru1 && sudo ip addr add 10.1.1.10/24 dev $TRU && sudo ip link set up dev $TRU

package main

import (
	"flag"
	"fmt"

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

	// Connect to tru
	var ifce *water.Interface
	t, err := Tru(*port, ifce, tru.Stat(*stat), tru.Hotkey(*hotkey),
		log, *loglevel, teolog.Logfilter(*logfilter))
	if err != nil {
		panic("can't create tru, error: " + err.Error())
	}
	defer t.Close()

	// Create tap interface
	_, err = Interface(*name, t)
	if err != nil {
		panic("can't create interface, error: " + err.Error())
	}
}

type TeoVpn struct {
}

// Tru create new tru connection
func Tru(port int, ifce *water.Interface, params ...interface{}) (t *tru.Tru, err error) {

	// Create server connection and start listen incominng packets
	t, err = tru.New(port, append(params,
		// Tru reader get all packets and resend it to interface
		func(ch *tru.Channel, pac *tru.Packet, err error) (processed bool) {
			if err != nil {
				log.Debug.Println("got error in main reader:", err)
				return
			}
			log.Debugv.Printf("got %d byte from %s, id %d: %s\n",
				pac.Len(), ch.Addr().String(), pac.ID(), pac.Data())
			ifce.Write(pac.Data())
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
func Interface(name string, t *tru.Tru) (ifce *water.Interface, err error) {
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
		t.ForEachChannel(func(ch *tru.Channel) { ch.WriteTo(frame) })
	}
}
