# Teonet TRU tunnel

Trutun creates secret tunnel by IP address using Teonet [TRU](https://github.com/teonet-go/tru) transport. [TRU](https://github.com/teonet-go/tru) create reliable, low latency and encrypted channel between connected peers.

[![GoDoc](https://godoc.org/github.com/teonet-go/trutun?status.svg)](https://godoc.org/github.com/teonet-go/trutun/)
[![Go Report Card](https://goreportcard.com/badge/github.com/teonet-go/trutun)](https://goreportcard.com/report/github.com/teonet-go/trutun)

## Usage example

Create regular tunnel between two hosts

Server:

```bash
# Start tunnel server
TRU=tru1 && sudo go run ./cmd/trutun -name=$TRU -p=9000 -loglevel=Debug -stat -hotkey

# Post connect commands, run it in other terminal
TRU=tru1 && sudo ip addr add 10.1.1.10/24 dev $TRU && sudo ip link set up dev $TRU
```

Client:

```bash
# Start tunnel client
TRU=tru2 && sudo go run ./cmd/trutun -name=$TRU -a=host.name:9000 -loglevel=Debug -stat -hotkey

# Post connect commands, run it in other terminal
TRU=tru2 && sudo ip addr add 10.1.1.11/24 dev $TRU && sudo ip link set up dev $TRU
```

You can simplify this commands by using `post connection` parameter and predefined shell script [if_up.sh](if_up.sh)

Server:

```bash
# Start tunnel server
TRU=tru1 && sudo go run ./cmd/trutun -name=$TRU -p=9000 -loglevel=Debug -stat -hotkey -pc="./if_up.sh $TRU 10.1.1.10/24"
```

Client:

```bash
# Start tunnel client
TRU=tru2 && sudo go run ./cmd/trutun -name=$TRU -a=host.name:9000 -loglevel=Debug -stat -hotkey -pc="./if_up.sh $TRU 10.1.1.11/24"
```

Parameters `-loglevel=Debug`, `-stat`, `-hotkey` are unnessesary and you can remove it from Start tunnel parameters. They are used to show statistic and log information.

## License

[BSD](LICENSE)
