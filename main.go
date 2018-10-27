package main

import (
	"fmt"
	"strconv"
	// "errors"

	"github.com/docopt/docopt-go"

	"github.com/EricQAQ/tikv-bench-tool/bench"
)

const VERSION = "v1.0.0"
const usage = `
Tikv Benchmark Command line Tool.

Usage:
    tibench [--pd-addr=<pd_addr>] [--op=<op>] [--total=<total>] [--worker=<worker_number>] [--data-size=<data_size>] [--client-type=<client_type>]
    tibench --help
    tibench --version

Options:
    --help                                          Show the help info
    --version                                       Show the bench tool version
    --pd-addr=<pd_addr>                             The PD(placement driver) server address, split by comma.
    --op=<op>                                       The tikv op to do benchtest.
    --total=<total>                                 The total request count.
    -w <worker_number>, --worker=<worker_number>    The number of the concurrent workers.
    --data-size=<data_size>                         The value data size(KB).
    --client-type=<client_type>                     The tikv client type, only "raw" or "trans".
`

func checkArguments(arguments *map[string]interface{}) error {
	if (*arguments)["--op"] == nil {
		(*arguments)["--op"] = "set"
	}
	if (*arguments)["--data-size"] == nil {
		(*arguments)["--data-size"] = 3
	}
	if (*arguments)["--worker"] == nil {
		(*arguments)["--worker"] = 10
	}
	if (*arguments)["--total"] == nil {
		(*arguments)["--total"] = 100000
	}
	if (*arguments)["--client-type"] == nil {
		(*arguments)["--client-type"] = "raw"
	}
	// if arguments["--client-type"] != "raw" || arguments["--client-type"] != "trans" {
	//     return errors.New("Client type is illegal. Only \"raw\" or \"trans\"")
	// }
	return nil
}

func main() {
	arguments, _ := docopt.Parse(usage, nil, true, VERSION, false)
	if err := checkArguments(&arguments); err != nil {
		return
	}
	addr := arguments["--pd-addr"].(string)
	op := arguments["--op"].(string)
	worker, _ := strconv.Atoi(arguments["--worker"].(string))
	total, _ := strconv.Atoi(arguments["--total"].(string))
	dataSize, _ := strconv.Atoi(arguments["--data-size"].(string))
	clientType, _ := arguments["--client-type"].(string)

	fmt.Printf(
		"Start Bench Test. PD Address: %s, " +
		"Operation: %s, Worker: %d, Total: %d, Client Type: %s, ValueSize: %dkb\n",
		addr, op, worker, total, clientType, dataSize,
	)

	client := bench.NewBenchClient(addr, total, worker, dataSize, clientType, op)
	client.StartBench()
}
