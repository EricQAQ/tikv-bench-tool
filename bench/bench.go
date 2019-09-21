package bench

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pingcap/tidb/config"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/store/tikv"
	"github.com/pingcap/tidb/terror"
	"gopkg.in/cheggaaa/pb.v2"
)

const TIKV_URL = "tikv://%s?cluster=1&disableGC=false"
const pbFmt = "{{ red \"%s\" }} " +
	"{{ bar . | green }} " +
	"Spend: {{ etime . | cyan }} " +
	"Remain: {{ rtime . | yellow }} " +
	"Speed: {{ speed . | blue }} " +
	"Count: {{ counters . | magenta }} " +
	"Percent: {{ percent . | white }}"
const report = "Finished Bench Test. \nTotal: %d requests.\nWorkers: %d.\nEach value size: %d.\n" +
	"==========================\n" +
	" 0 ~ 3ms		: %d\n" +
	" 3 ~ 5ms		: %d\n" +
	" 5 ~ 10ms		: %d\n" +
	"10 ~ 15ms		: %d\n" +
	"15 ~ 20ms		: %d\n" +
	"20 ~ 30ms		: %d\n" +
	"30 ~ 50ms		: %d\n" +
	"50 ~ 500ms		: %d\n" +
	"500~ max ms		: %d\n" +
	"==========================\n" +
	"Success: %d, Failed: %d, Success Rate: %f\n"

var wg sync.WaitGroup

func makeData(kb int) *[]byte {
	data := make([]byte, kb*1024)
	for i := 0; i < kb*1024; i++ {
		data = append(data, byte('x'))
	}
	return &data
}

type BenchClient struct {
	tikvTransClient kv.Storage
	tikvRawClient   *tikv.RawKVClient
	total           int
	worker          int
	chCnt           chan int
	clientType      string
	size            int
	value           *[]byte
	cmd             string
	timeRange       []int32
	succCount       int32
	failCount       int32
	benchBar        *pb.ProgressBar
}

// NewBenchClient creates a BenchClient
func NewBenchClient(url string, total, worker, size int, clientType, cmd string) *BenchClient {
	c := BenchClient{
		total:      total,
		worker:     worker,
		chCnt:      make(chan int, 1024),
		clientType: clientType,
		size:       size,
		value:      makeData(size),
		cmd:        strings.ToLower(cmd),
		timeRange:  []int32{0, 0, 0, 0, 0, 0, 0, 0, 0},
		succCount:  0,
		failCount:  0,
		benchBar:   pb.ProgressBarTemplate(fmt.Sprintf(pbFmt, "TiBenchmark")).Start(total),
	}
	c.OpenTikv(url)
	return &c
}

func (client *BenchClient) OpenTikv(tikvURL string) {
	if client.clientType == "raw" {
		store, err := tikv.NewRawKVClient(strings.Split(tikvURL, ","), config.Security{})
		terror.MustNil(err)
		client.tikvRawClient = store
	} else {
		driver := tikv.Driver{}
		store, err := driver.Open(fmt.Sprintf(TIKV_URL, tikvURL))
		terror.MustNil(err)
		client.tikvTransClient = store
	}
}

func (client *BenchClient) randomKey() []byte {
	b := make([]byte, 8)
	return EncodeInt(b, int64(rand.Int()))
}

func (client *BenchClient) updateTimeRange(duration int64) {
	switch {
	case duration >= 0 && duration <= 3:
		atomic.AddInt32(&client.timeRange[0], 1)
	case duration > 3 && duration <= 5:
		atomic.AddInt32(&client.timeRange[1], 1)
	case duration > 5 && duration <= 10:
		atomic.AddInt32(&client.timeRange[2], 1)
	case duration > 10 && duration <= 15:
		atomic.AddInt32(&client.timeRange[3], 1)
	case duration > 15 && duration <= 20:
		atomic.AddInt32(&client.timeRange[4], 1)
	case duration > 20 && duration <= 30:
		atomic.AddInt32(&client.timeRange[5], 1)
	case duration > 30 && duration <= 50:
		atomic.AddInt32(&client.timeRange[6], 1)
	case duration > 50 && duration <= 500:
		atomic.AddInt32(&client.timeRange[7], 1)
	case duration > 500:
		atomic.AddInt32(&client.timeRange[8], 1)
	}
}

func (client *BenchClient) opInTxn(fn func(txn kv.Transaction) (interface{}, error), write bool) {
	txn, err := client.tikvTransClient.Begin()
	if err != nil {
		atomic.AddInt32(&client.failCount, 1)
		return
	}
	if _, err = fn(txn); err != nil {
		atomic.AddInt32(&client.failCount, 1)
		return
	}
	if !write {
		atomic.AddInt32(&client.succCount, 1)
		return
	}
	if err = txn.Commit(context.Background()); err != nil {
		txn.Rollback()
		atomic.AddInt32(&client.failCount, 1)
		return
	}
	atomic.AddInt32(&client.succCount, 1)
}

func (client *BenchClient) rawGet() {
	_, err := client.tikvRawClient.Get(client.randomKey())
	if err != nil {
		atomic.AddInt32(&client.failCount, 1)
		return
	}
}

func (client *BenchClient) transGet() {
	fn := func(txn kv.Transaction) (interface{}, error) {
		return txn.Get(client.randomKey())
	}
	client.opInTxn(fn, false)
}

func (client *BenchClient) rawSet() {
	err := client.tikvRawClient.Put(client.randomKey(), *client.value)
	if err != nil {
		atomic.AddInt32(&client.failCount, 1)
		return
	}
}

func (client *BenchClient) transSet() {
	fn := func(txn kv.Transaction) (interface{}, error) {
		err := txn.Set(client.randomKey(), *client.value)
		return nil, err
	}
	client.opInTxn(fn, true)
}

func (client *BenchClient) benchGet() {
	startTs := time.Now().UnixNano() / 1e6
	if client.clientType == "raw" {
		client.rawGet()
	} else {
		client.transGet()
	}
	duration := time.Now().UnixNano()/1e6 - startTs
	client.updateTimeRange(duration)
	atomic.AddInt32(&client.succCount, 1)
}

func (client *BenchClient) benchSet() {
	startTs := time.Now().UnixNano() / 1e6
	if client.clientType == "raw" {
		client.rawSet()
	} else {
		client.transSet()
	}
	duration := time.Now().UnixNano()/1e6 - startTs
	client.updateTimeRange(duration)
	atomic.AddInt32(&client.succCount, 1)
}

func (client *BenchClient) getTask() {
	for i := client.total; i > 0; i-- {
		client.chCnt <- 1
	}
	close(client.chCnt)
}

func (client *BenchClient) deferTask() {
	client.benchBar.Finish()

	if client.clientType == "raw" {
		client.tikvRawClient.Close()
	}
	fmt.Printf(
		report, client.total, client.worker, client.size,
		client.timeRange[0], client.timeRange[1],
		client.timeRange[2], client.timeRange[3],
		client.timeRange[4], client.timeRange[5],
		client.timeRange[6], client.timeRange[7],
		client.timeRange[8], client.succCount,
		client.failCount, float32(client.succCount)/float32(client.total),
	)
}

func (client *BenchClient) doTask() {
	defer wg.Done()

	for range client.chCnt {
		switch client.cmd {
		case "get":
			client.benchGet()
		case "set":
			client.benchSet()
		}
		client.benchBar.Add(1)
	}
}

func (client *BenchClient) StartBench() {
	defer client.deferTask()
	go client.getTask()

	for i := 0; i < client.worker; i++ {
		wg.Add(1)
		go client.doTask()
	}
	wg.Wait()
}
